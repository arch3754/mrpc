package server

type Plugin interface {
	ServiceRegister() error
	Close() error
}