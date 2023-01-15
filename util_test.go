package main

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func Test_validateUriParamUrl(t *testing.T) {
	type args struct {
		c *gin.Context
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "valid", args: args{&gin.Context{Request: httptest.NewRequest("GET", "http://localhost:8080?url=http://localhost/pdf.pdf", nil)}}, want: "http://localhost/pdf.pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateUriParamUrl(tt.args.c); got != tt.want {
				t.Errorf("validateUriParamUrl() = %v, want %v", got, tt.want)

			}
		})
	}
}
