package adapter

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/miekg/dns"
)

type apiResponse struct {
	Status   int  // Standard DNS response code (32 bit integer).
	TC       bool // Whether the response is truncated
	RD       bool // Always true for Google Public DNS
	RA       bool // Always true for Google Public DNS
	AD       bool // Whether all response data was validated with DNSSEC
	CD       bool // Whether the client asked to disable DNSSEC
	Question []map[string]interface{}
	Answer   []apiResponseAnswer
}

type apiResponseAnswer struct {
	Name string `json:name`
	Type uint16 `json:type`
	TTL  int
	Data string `json:data`
}

func (a *apiResponseAnswer) String() string {
	v := dns.Fqdn(a.Name) + "\t"
	v += strconv.Itoa(a.TTL) + "\t"
	v += dns.ClassToString[dns.ClassINET] + "\t"
	v += dns.TypeToString[a.Type] + "\t"
	v += a.Data

	return v
}

type Handle struct {
	API string
}

func (h *Handle) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)

	for _, q := range r.Question {
		hdr, err := h.ResolveByHttp(q.Name, q.Qtype)
		if err != nil || hdr.Status != 0 {
			dns.HandleFailed(w, r)
			return
		}

		msg.Authoritative = hdr.TC

		for _, a := range hdr.Answer {
			if rr, err := dns.NewRR(a.String()); err == nil {
				msg.Answer = append(msg.Answer, rr)
			}
		}
	}

	w.WriteMsg(&msg)
}

func (h *Handle) ResolveByHttp(name string, rtype uint16) (*apiResponse, error) {
	var (
		hdr *apiResponse
		err error
	)

	client := &http.Client{}

	req, err := http.NewRequest("GET", h.API, nil)
	if err != nil {
		log.Println("Unable to make request:", err.Error())
		return hdr, err
	}

	q := req.URL.Query()
	q.Add("name", name)
	q.Add("type", strconv.Itoa(int(rtype)))
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		log.Println("HTTP DNS API Error:", err.Error())
		return hdr, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("Unable to read response body:", err.Error())
		return hdr, err
	}

	err = json.Unmarshal(body, &hdr)
	if err != nil {
		log.Println("Unable to parse response as JSON:", err.Error())
		return hdr, err
	}

	return hdr, nil
}
