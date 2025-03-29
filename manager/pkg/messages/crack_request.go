package messages

import "encoding/xml"

type CrackHashManagerRequest struct {
	XMLName    xml.Name `xml:"CrackHashManagerRequest"`
	RequestId  string   `xml:"RequestId"`
	PartNumber int      `xml:"PartNumber"`
	PartCount  int      `xml:"PartCount"`
	Hash       string   `xml:"Hash"`
	MaxLength  int      `xml:"MaxLength"`
	Alphabet   Alphabet `xml:"Alphabet"`
}

type Alphabet struct {
	Symbols []string `xml:"symbols"`
}

type CrackHashWorkerResponse struct {
	XMLName   xml.Name `xml:"CrackHashWorkerResponse"`
	RequestId string   `xml:"RequestId"`
	Found     []string `xml:"Found>Value"`
}
