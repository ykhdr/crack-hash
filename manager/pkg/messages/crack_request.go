package messages

import "encoding/xml"

type CrackHashManagerRequest struct {
	XMLName    xml.Name `xml:"CrackHashManagerRequest" bson:"-"`
	RequestId  string   `xml:"RequestId" bson:"RequestId"`
	PartNumber int      `xml:"PartNumber" bson:"PartNumber"`
	PartCount  int      `xml:"PartCount" bson:"PartCount"`
	Hash       string   `xml:"Hash" bson:"Hash"`
	MaxLength  int      `xml:"MaxLength" bson:"MaxLength"`
	Alphabet   Alphabet `xml:"Alphabet" bson:"Alphabet"`
}

type Alphabet struct {
	Symbols []string `xml:"symbols" bson:"symbols"`
}

type CrackHashWorkerResponse struct {
	XMLName   xml.Name `xml:"CrackHashWorkerResponse" bson:"-"`
	Id        string   `xml:"-" bson:"_id"`
	RequestId string   `xml:"RequestId" bson:"request_id"`
	Found     []string `xml:"Found>Value" bson:"found"`
}
