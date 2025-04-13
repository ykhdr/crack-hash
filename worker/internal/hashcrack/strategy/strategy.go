package strategy

import "github.com/ykhdr/crack-hash/manager/pkg/messages"

type CrackResult interface {
	Found() []string
}

type crackResult struct {
	found []string
}

func (r *crackResult) Found() []string {
	return r.found
}

type Strategy interface {
	CrackMd5(req *messages.CrackHashManagerRequest) CrackResult
}
