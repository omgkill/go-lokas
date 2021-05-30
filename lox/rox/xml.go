package rox

import "encoding/xml"
import "net/http"

func XML(w ResponseWriter, val interface{}, code ...int) {
	var b []byte
	var err error

	if Pretty {
		b, err = xml.MarshalIndent(val, "", "  ")
	} else {
		b, err = xml.Marshal(val)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml")

	if len(code) > 0 {
		w.WriteHeader(code[0])
	}

	w.Write(b)
}
