package pushstream

import (
	"crypto/sha1"
	"fmt"
)

type PushNotification struct {
	Brdname   string `json:"brdname"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	Signature string `json:"sig"`
}

func (p *PushNotification) Sign(secret string) {
	p.Signature = p.calcSig(secret)
}

func (p *PushNotification) CheckSignature(secret string) bool {
	return p.Signature == p.calcSig(secret)
}

func (p *PushNotification) calcSig(secret string) string {
	return sha1hex(fmt.Sprintf("%v/%v/%v/%v", p.Brdname, p.Filename, p.Size, secret))
}

func GetPushChannel(p *PushNotification, secret string) string {
	return sha1hex(fmt.Sprintf("%v/%v/%v", p.Brdname, p.Filename, secret))
}

func sha1hex(s string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(s)))
}
