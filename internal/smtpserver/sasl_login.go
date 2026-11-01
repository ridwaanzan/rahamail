package smtpserver

import (
	"errors"

	"github.com/emersion/go-sasl"
)

var (
	challengeUsername = []byte("Username:")
	challengePassword = []byte("Password:")
)

// LoginAuthenticator mendefinisikan fungsi autentikasi untuk SASL LOGIN
type LoginAuthenticator func(username, password string) error

type loginServer struct {
	step         int
	username     string
	authenticate LoginAuthenticator
}

// Next mengimplementasikan alur 2-step challenge-response untuk mekanisme SASL LOGIN:
// Step 0: Server mengirim challenge "Username:"
// Step 1: Client mengirim username, Server membalas challenge "Password:"
// Step 2: Client mengirim password, Server memverifikasi kredensial
func (s *loginServer) Next(response []byte) (challenge []byte, done bool, err error) {
	switch s.step {
	case 0:
		// Client mengirim initial response berisi username atau kosong
		if len(response) > 0 {
			s.username = string(response)
			s.step = 2
			return challengePassword, false, nil
		}
		s.step = 1
		return challengeUsername, false, nil

	case 1:
		s.username = string(response)
		s.step = 2
		return challengePassword, false, nil

	case 2:
		password := string(response)
		if s.authenticate == nil {
			return nil, true, errors.New("sasl: missing authenticator")
		}
		err = s.authenticate(s.username, password)
		if err != nil {
			return nil, true, err
		}
		return nil, true, nil

	default:
		return nil, true, sasl.ErrUnexpectedClientResponse
	}
}

// NewLoginServer membuat server SASL LOGIN mechanism sesuai spesifikasi draft-murchison-sasl-login
func NewLoginServer(authenticator LoginAuthenticator) sasl.Server {
	return &loginServer{
		step:         0,
		authenticate: authenticator,
	}
}

// Verify that loginServer implements sasl.Server
var _ sasl.Server = (*loginServer)(nil)
