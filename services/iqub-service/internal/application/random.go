package application

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
)

// Invite codes avoid look-alike characters (no 0/O, 1/I).
const (
	inviteAlphabet   = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	inviteCodeLength = 6
)

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// The system random source failing is not recoverable.
		panic("cannot generate id: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func randomInviteCode() string {
	code := make([]byte, inviteCodeLength)
	for k := range code {
		code[k] = inviteAlphabet[randomInt(len(inviteAlphabet))]
	}
	return string(code)
}

// shuffled returns a randomly ordered copy and never changes the input.
func shuffled(items []string) []string {
	out := append([]string(nil), items...)
	for k := len(out) - 1; k > 0; k-- {
		j := randomInt(k + 1)
		out[k], out[j] = out[j], out[k]
	}
	return out
}

func randomInt(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic("cannot generate random number: " + err.Error())
	}
	return int(v.Int64())
}
