package api

import "encoding/xml"

// Это структура запроса от менеджера к воркеру (аналог CrackHashManagerRequest из XSD)
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
	// в XSD <xs:element name="symbols" type="xs:string" minOccurs="0" maxOccurs="unbounded"/>
}

// Ответ, который воркер присылает менеджеру
type CrackHashWorkerResponse struct {
	XMLName   xml.Name `xml:"CrackHashWorkerResponse"`
	RequestId string   `xml:"RequestId"`
	Found     []string `xml:"Found>Value"`
	// аналогично: массив строк, каждая в отдельном тэге <Value> внутри <Found>
}
