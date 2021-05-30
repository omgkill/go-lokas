package stringutil

import (
	"github.com/nomos/go-log/log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

func StartWithCapital(str string)bool {
	c1:=str[0]
	return c1>64&&c1<91
}


func FirstToUpper(str string) string {
	var upperStr string
	vv := []rune(str)   // 后文有介绍
	for i := 0; i < len(vv); i++ {
		if i == 0 {
			if vv[i] >= 97 && vv[i] <= 122 {  // 后文有介绍
				vv[i] -= 32 // string的码表相差32位
				upperStr += string(vv[i])
			} else {
				return str
			}
		} else {
			upperStr += string(vv[i])
		}
	}
	return upperStr
}


func CamelToUnder(str string) string {
	var upperStr string
	vv := []rune(str)   // 后文有介绍
	for i := 0; i < len(vv); i++ {
		if vv[i] >= 65 && vv[i] <= 90 {  // 后文有介绍
			if i!=0 {
				upperStr+="_"
			}
			vv[i] += 32 // string的码表相差32位
			upperStr += string(vv[i])
		} else {
			upperStr += string(vv[i])
		}
	}
	return upperStr
}

func FirstToLower(str string) string {
	var upperStr string
	vv := []rune(str)   // 后文有介绍
	for i := 0; i < len(vv); i++ {
		if i == 0 {
			if vv[i] >= 65 && vv[i] <= 90 {  // 后文有介绍
				vv[i] += 32 // string的码表相差32位
				upperStr += string(vv[i])
			} else {
				return str
			}
		} else {
			upperStr += string(vv[i])
		}
	}
	return upperStr
}


func RandString(len int)string {
	if len<=0 {
		len = 1
	}
	list:="ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	ret:=""
	for i:=0;i<len;i++ {
		if i==0 {
			ret+=string(list[rand.Intn(51)])
		} else {
			ret+=string(list[rand.Intn(61)])
		}
	}
	log.Warnf("str",ret)
	return ret
}

func AddStringGap (str string,min int,gap int)string {
	delta2 := gap - len(str)%gap
	for {
		if len(str)+delta2 < min {
			delta2 += gap
			continue
		}
		break
	}
	for i := 0; i < delta2; i++ {
		str += " "
	}
	return str
}

func LocalPath()string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Error(err.Error())
		return ""
	}
	return dir
}

func CopyString(str string)string {
	data:=[]byte(str)
	data1:=make([]byte,len(data))
	copy(data1,data)
	return string(data1)
}

func SplitCamelCase(src string) []string {
	// don't split invalid utf8
	if !utf8.ValidString(src) {
		return []string{src}
	}
	entries := []string{}
	var runes [][]rune
	lastClass := 0
	class := 0
	// split into fields based on class of unicode character
	for _, r := range src {
		switch true {
		case unicode.IsLower(r):
			class = 1
		case unicode.IsUpper(r):
			class = 2
		case unicode.IsDigit(r):
			class = 3
		default:
			class = 4
		}
		if class == lastClass {
			runes[len(runes)-1] = append(runes[len(runes)-1], r)
		} else {
			runes = append(runes, []rune{r})
		}
		lastClass = class
	}
	// handle upper case -> lower case sequences, e.g.
	// "PDFL", "oader" -> "PDF", "Loader"
	for i := 0; i < len(runes)-1; i++ {
		if unicode.IsUpper(runes[i][0]) && unicode.IsLower(runes[i+1][0]) {
			runes[i+1] = append([]rune{runes[i][len(runes[i])-1]}, runes[i+1]...)
			runes[i] = runes[i][:len(runes[i])-1]
		}
	}
	// construct []string from results
	for _, s := range runes {
		if len(s) > 0 {
			entries = append(entries, string(s))
		}
	}
	ret:=[]string{}
	for _,s:=range entries {
		if regexp.MustCompile(`[_]*`).FindString(s)!=s {
			ret = append(ret, s)
		}
	}
	return ret
}

func SplitCamelCaseUpper(src string) (entries []string) {
	ret:=SplitCamelCase(src)
	for i,v:=range ret {
		ret[i] = strings.TrimSpace(strings.ToUpper(v))
	}
	return ret
}

func Capitalize(str string) string {
	var upperStr string
	vv := []rune(str)   // 后文有介绍
	for i := 0; i < len(vv); i++ {
		if i == 0 {
			if vv[i] >= 97 && vv[i] <= 122 {  // 后文有介绍
				vv[i] -= 32 // string的码表相差32位
				upperStr += string(vv[i])
			} else {
				return str
			}
		} else {
			upperStr += string(vv[i])
		}
	}
	return upperStr
}

func SplitCamelCaseCapitalize(src string) (entries []string) {
	ret:=SplitCamelCase(src)
	for i,v:=range ret {
		ret[i] = strings.TrimSpace(Capitalize(v))
	}
	return ret
}

func SplitCamelCaseCapitalizeSlash(src string) string {
	ret:=SplitCamelCaseCapitalize(src)
	return strings.Join(ret,"_")
}

func SplitCamelCaseUpperSlash(src string) string {
	ret:=SplitCamelCaseUpper(src)
	return strings.Join(ret,"_")
}

func SplitCamelCaseLower(src string) (entries []string) {
	ret:=SplitCamelCase(src)
	for i,v:=range ret {
		ret[i] = strings.TrimSpace(strings.ToLower(v))
	}
	return ret
}

func SplitCamelCaseLowerSlash(src string) string {
	ret:=SplitCamelCaseLower(src)
	return strings.Join(ret,"_")
}