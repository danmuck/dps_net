package main

import (
	"fmt"
	"log"
)

type FatalErr struct {
	Err error
}

func (e FatalErr) Unwrap() error { return e.Err }

func (e FatalErr) Error() string {
	return "exit error"
}

func FatalError(err error) error {
	return FatalErr{Err: err}
}

func FatalErrorStr(msg string, args ...any) error {
	return FatalError(fmt.Errorf(msg, args...))
}

type LogErr struct {
	Err error
}

func (e LogErr) Unwrap() error { return e.Err }

func (e LogErr) Error() string {
	return "log error"
}

func LogError(err error) error {
	log.Println(err)
	return LogErr{Err: err}
}

func LogErrorStr(msg string, args ...any) error {
	return LogError(fmt.Errorf(msg, args...))
}
