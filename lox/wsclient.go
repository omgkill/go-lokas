package lox

import (
	"context"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/nomos/go-lokas"
	"github.com/nomos/go-lokas/log"
	"github.com/nomos/go-lokas/network"
	"github.com/nomos/go-lokas/protocol"
	"github.com/nomos/go-lokas/util/events"
	"github.com/nomos/go-lokas/util/promise"
	"go.uber.org/zap"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var _ lokas.INetClient = (*WsClient)(nil)

type WsClient struct {
	events.EventEmmiter
	*ActiveSession
	conn           *websocket.Conn
	ws             *wsImpl
	timeout        time.Duration
	addr           string
	idGen          uint32
	context lokas.IReqContext
	reqContexts    map[uint32]lokas.IReqContext
	isOpen         bool
	Closing        bool
	Opening        bool
	done           chan struct{}
	contextMutex   sync.Mutex
	openingPending *promise.Promise
	closePending   *promise.Promise
}

func NewWsClient() *WsClient {
	ret := &WsClient{
		EventEmmiter: events.New(),
		context: network.NewDefaultContext(context.TODO()),
		reqContexts:  make(map[uint32]lokas.IReqContext),
		timeout:      TimeOut,
		isOpen:       false,
		ActiveSession: NewActiveSession(nil,0,nil),
	}
	ret.MsgHandler = ret.MessageHandler
	ret.init()
	return ret
}

func (this *WsClient) init() {

}

func (this *WsClient) genId() uint32 {
	this.idGen++
	return this.idGen
}

func (this *WsClient) SetProtocol(p protocol.TYPE) {
	this.Protocol = p
	if this.ActiveSession!=nil {
		this.ActiveSession.Protocol = p
	}
}

func (this *WsClient) Connected() bool {
	return this.isOpen
}

func (this *WsClient) MessageHandler(msg *protocol.BinaryMessage){
	id,_:=msg.GetId()
	log.Warnf("MessageHandler",id.String(),msg.TransId,id)
	if msg.TransId!=0 {
		ctx:=this.GetContext(msg.TransId)
		ctx.SetResp(msg.Body)
		ctx.Finish()
	}
}

func (this *WsClient) Connect(addr string) *promise.Promise {
	addr = "ws://" + addr + "/ws"
	if this.addr != "" && this.addr != addr {
		return this.Close().Catch(func(err error) interface{} {
			log.Error(err.Error())
			return nil
		}).Then(func(data interface{}) interface{} {
			this.addr = addr
			return this.Open()
		})
	}
	this.addr = addr
	return this.Open()
}

func (this *WsClient) ClearContext() {
	this.contextMutex.Lock()
	defer this.contextMutex.Unlock()
	for _, v := range this.reqContexts {
		v.Cancel(errors.New("clear context"))
	}
}

func (this *WsClient) Disconnect(force bool) *promise.Promise {
	if this.isOpen {
		this.isOpen = false

		this.Closing = true
		return this.Close()
	}
	if force {
		if this.openingPending != nil {
			this.openingPending.Reject("force disconnect")
		}
	}
	return this.Close()
}

func (this *WsClient) Run() {
	this.done = make(chan struct{}, 0)
	go func() {
		for {
			select {

			case <-this.done:
				return
			}
		}
	}()
}

func (this *WsClient) onerror() {

}

func (this *WsClient) Open() *promise.Promise {
	if this.isOpen {
		return promise.Resolve(nil)
	}
	if this.openingPending == nil {
		this.openingPending = promise.Async(func(resolve func(interface{}), reject func(interface{})) {
			timeout := promise.SetTimeout(TimeOut, func(timeout *promise.Timeout) {
				reject("timeout")
				this.openingPending = nil
			})
			this.Opening = true
			ws, err := NewWebSocket(this.addr, this)
			if err != nil {
				log.Error("create ws error", zap.String("err", err.Error()))
				reject(err)
				this.openingPending = nil
				return
			}
			this.ws = ws

			this.Once("open", func(i ...interface{}) {
				timeout.Close()
				this.openingPending = nil
				resolve(nil)
			})
		})

	}
	return this.openingPending
}

func (this *WsClient) Close() *promise.Promise {
	this.isOpen = false
	if this.closePending == nil {
		this.closePending = promise.Async(func(resolve func(interface{}), reject func(interface{})) {
			if this.ws == nil {
				resolve(nil)
				this.closePending = nil
				return
			}
			_, err := this.ws.Close().Await()
			if err != nil {
				reject(err)
				this.openingPending = nil
				return
			}
			this.Closing = false
			resolve(nil)
			this.closePending = nil
		})
	}
	return this.closePending
}

func (this *WsClient) OnOpen(conn *websocket.Conn) {
	this.conn = this.ws.Conn
	this.Opening = false
	log.Warn(this.addr + " connected")
	this.isOpen = true
	this.Emit("open")
}

func (this *WsClient) OnClose(conn *websocket.Conn) {
	if this.isOpen {
		this.Disconnect(true)
	}
	this.conn = nil
	this.ws = nil
	this.Closing = false
	log.Warn(this.addr + " disconnected")
	this.Emit("close")
}

func (this *WsClient) OnMessage(cmdId protocol.BINARY_TAG, listeners ...events.Listener) {
	this.On(events.EventName("CmdId"+strconv.Itoa(int(cmdId))), listeners...)
}

func (this *WsClient) Off(cmdId uint16, listener events.Listener) {
	if listener != nil {
		this.RemoveListener(events.EventName("CmdId"+strconv.Itoa(int(cmdId))), listener)
	} else {
		this.RemoveAllListeners(events.EventName("CmdId" + strconv.Itoa(int(cmdId))))
	}
}

func (this *WsClient) SendMessage(transId uint32, msg interface{}) {
	if this.Protocol == protocol.JSON {
		this.sendJsonMessage(transId,msg)
	} else if this.Protocol == protocol.BINARY {
		this.sendBinaryMessage(transId,msg)
	} else {
		panic("unidentified protocol")
	}
}



func (this *WsClient) sendJsonMessage(transId uint32, msg interface{}) {
	rb, err := protocol.MarshalJsonMessage(transId, msg)
	if err != nil {
		log.Error(err.Error())
		return
	}
	this.conn.WriteMessage(websocket.BinaryMessage,rb)
}

func (this *WsClient) sendBinaryMessage(transId uint32, msg interface{}) {
	rb, err := protocol.MarshalBinaryMessage(transId, msg)
	if err != nil {
		log.Error(err.Error())
		return
	}
	this.conn.WriteMessage(websocket.BinaryMessage,rb)
}

func (this *WsClient) addContext(transId uint32, ctx lokas.IReqContext) {
	this.contextMutex.Lock()
	defer this.contextMutex.Unlock()
	this.reqContexts[transId] = ctx
}

func (this *WsClient) removeContext(transId uint32) {
	this.contextMutex.Lock()
	defer this.contextMutex.Unlock()
	delete(this.reqContexts, transId)
}

func (this *WsClient) GetContext(transId uint32) lokas.IReqContext {
	this.contextMutex.Lock()
	defer this.contextMutex.Unlock()
	return this.reqContexts[transId]
}


func (this *WsClient) SetMessageHandler(handler func(msg *protocol.BinaryMessage)) {
	this.MsgHandler = handler
}

func (this *WsClient) Request(req interface{}) *promise.Promise {
	return promise.Async(func(resolve func(interface{}), reject func(interface{})) {
		if this.Opening {
			_, err := this.Open().Await()
			if err != nil {
				reject(err)
				return
			}
		} else if !this.Connected() {
			//log.Warn("connection closed",this)
			reject(errors.New("connection closed"))
			return
		}
		id := this.genId()
		resp,err := this.Call(id, req)
		if err != nil {
			log.Error("Call Error:%s", zap.String("err", err.Error()))
			reject(err)
			return
		}
		resolve(resp)
	})
}

func (this *WsClient) Call(transId uint32, req interface{}) (interface{},error) {
	ctx := network.NewDefaultContextWithTimeout(this.context,transId,this.timeout)
	return this.doCall(ctx, req,true)
}

func (this *WsClient) AsyncCall(transId uint32, req interface{}) (interface{},error) {
	ctx := network.NewDefaultContextWithTimeout(this.context,transId,this.timeout)
	return this.doCall(ctx, req,false)
}

func (this *WsClient) doCall(ctx lokas.IReqContext, req interface{}, isSync bool) (interface{},error) {
	transId := ctx.GetTransId()
	this.addContext(transId, ctx)
	//cmdId, err := protocol.GetCmdIdFromType(req)
	//if err != nil {
	//	log.Error(err.Error())
	//	return err
	//}
	rb, err := protocol.MarshalBinaryMessage(transId, req)
	if err != nil {
		log.Error(err.Error())
		return nil,err
	}
	this.ws.writeChan <- rb
	if !isSync {
		return nil,nil
	}
	select {
	case <-ctx.Done():
		switch ctx.Err() {
		case context.DeadlineExceeded:
			this.removeContext(transId)
			if this.isOpen {
				this.Close().Await()
				go func() {
					this.Open().Await()
				}()
			}
			return nil,protocol.ERR_RPC_TIMEOUT
		default:
			resp:=ctx.GetResp()
			if err,ok:=resp.(*protocol.ErrMsg);ok {
				return nil,err
			}
			return ctx.GetResp(),nil
		}
	}
}


func (this *WsClient) OnRecv(connect lokas.IConn,data []byte) {
	_, err := this.HookRecv(data)
	if err != nil {
		log.Error("WebSocket OnRecv err:" + err.Error())
	}
}

func (this *WsClient) OnRecvData(data []byte) {
	_, err := this.HookRecv(data)
	if err != nil {
		log.Error("WebSocket OnRecv err:" + err.Error())
	}
}

func (this *WsClient) OnRecvMessage(cmdId protocol.BINARY_TAG, transId uint32, msg interface{}) {
	this.Emit(events.EventName("CmdId"+strconv.Itoa(int(cmdId))), msg, transId)
}

func (this *WsClient) OnRecvCmd(cmdId protocol.BINARY_TAG, time time.Duration) *promise.Promise {
	return promise.Async(func(resolve func(interface{}), reject func(interface{})) {
		timeout := promise.SetTimeout(time, func(timeout *promise.Timeout) {
			reject("timeout")
		})
		this.Once(events.EventName("CmdId"+strconv.Itoa(int(cmdId))), func(i ...interface{}) {
			timeout.Close()
			msg := i[0]
			resolve(msg)
			return
		})
	})
}

func (this *WsClient) HookRecv(data []byte) (interface{}, error) {
	msg, err := protocol.UnmarshalBinaryMessage(data)
	if err != nil {
		return nil, err
	}
	if msg.TransId == 0 {
		this.OnRecvMessage(msg.CmdId, msg.TransId, msg.Body)
		return msg, nil
	}
	if msg.TransId != 0 {
		ctx := this.GetContext(msg.TransId)
		if ctx == nil {
			log.Error("msgCmdId:%d TransId :%d ctx not found",
				zap.Any("cmdId", msg.CmdId),
				zap.Uint32("transId", msg.TransId),
			)
			return msg, errors.New(fmt.Sprintf("msgCmdId:%d TransId :%d ctx not found", msg.CmdId, msg.TransId))
		}
		if msg.CmdId == protocol.TAG_Error {
			log.Error("CmdErrorAckId")
			body := msg.Body.(*protocol.ErrMsg)
			ctx.Cancel(&protocol.ErrMsg{
				Code:    body.Code,
				Message: "code:" + strconv.Itoa(int(body.Code)) + "," + body.Message,
			})
			return nil, err
		}
		this.removeContext(ctx.GetTransId())
		ctx.SetResp(msg.Body)
		ctx.Finish()
		this.OnRecvMessage(msg.CmdId, msg.TransId, msg.Body)
		return msg, nil
	}
	return nil, nil
}

const (
	HeaderSize = 8
	ProtectLongPacketSize = 4 * 1024 * 1024
)

type wsImpl struct {
	*websocket.Conn
	client         *WsClient
	writeChan      chan []byte
	wg             sync.WaitGroup
	closeOnce      sync.Once
	longPacketData []byte
	done           chan struct{}
	once           sync.Once
	closing        bool
}

func NewWebSocket(url string,client *WsClient) (*wsImpl, error) {
	ret := &wsImpl{
		Conn:      nil,
		writeChan: make(chan []byte),
	}

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}
	ret.client = client
	ret.Conn = conn
	ret.ServeIO()
	return ret, nil
}

const (
	writeWait = 10 * time.Second
	pongWait = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMessageSize = 1024*1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024*1024,
	WriteBufferSize: 1024*1024*1024,
	CheckOrigin: func(r *http.Request) bool {
		return strings.HasPrefix(r.RemoteAddr, "127.0.0.1") || r.Header["Origin"][0] == r.Host
	},
}

func (this *wsImpl) ServeIO() {
	this.wg.Add(2)
	this.done = make(chan struct{})
	go func() {
		this.writePump()
		this.wg.Done()
	}()

	go func() {
		this.client.OnOpen(this.client.conn)
		this.readPump()
		this.client.OnClose(this.client.conn)
		this.wg.Done()
	}()
}

func (this *wsImpl) readPump() {
	defer func() {
		this.Conn.Close()
	}()

	this.Conn.SetReadLimit(maxMessageSize)
	this.Conn.SetReadDeadline(time.Now().Add(pongWait))
	this.Conn.SetPongHandler(func(string) error {
		this.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		select {
		case <-this.done :
			return
		default:
			_, message, err := this.Conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
					log.Error("error: %v", zap.Error(err))
				}
				return
			}
			data := this.readLongPacket(message)
			this.client.OnRecvData(data)
		}
	}
}

func (this *wsImpl) readLongPacket(data []byte) []byte {
	isLongPacket, idx, packetData := protocol.PickBinaryLongPacket(data)
	if !isLongPacket {
		return data
	}

	this.longPacketData = append(this.longPacketData, packetData...)
	if idx == 0 {
		data := this.longPacketData[:]
		this.longPacketData = nil
		return data
	}

	//protect too long
	if len(this.longPacketData) > ProtectLongPacketSize {
		log.Error("protect too long")
		this.longPacketData = nil
	}
	return nil
}

func (this *wsImpl) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		this.Conn.Close()
	}()

	for {
		select {
		case <-this.done :
			return
		case res, ok := <-this.writeChan:
			this.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				this.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			//data := make([]byte, len(res))
			//copy(data, res)
			//log.Warn("send res",len(data))
			//err := this.Conn.WriteMessage(websocket.BinaryMessage,data)
			//if err != nil {
			//	return
			//}

			w, err := this.Conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				return
			}
			w.Write(res)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			this.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := this.Conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func (this *wsImpl) Close() *promise.Promise {
	return promise.Async(func(resolve func(interface{}), reject func(interface{})) {
		if this.closing {
			for {
				time.Sleep(time.Millisecond*50)
				if this.closing == false {
					resolve(nil)
					return
				}
			}
		} else {
			if this.done!=nil {
				this.done <- struct{}{}
				close(this.done)
			}
			this.wg.Wait()
			resolve(nil)
		}
	})
}


