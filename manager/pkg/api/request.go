package api

type CrackRequest struct {
	Hash      string `json:"hash" bson:"hash"`
	MaxLength int    `json:"maxLength" bson:"max_length"`
}
