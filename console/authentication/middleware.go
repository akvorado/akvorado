// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package authentication

import (
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// UserInformation contains information about the current user.
type UserInformation struct {
	Login     string `json:"login" header:"LOGIN" binding:"required"`
	Name      string `json:"name,omitempty" header:"NAME"`
	Email     string `json:"email,omitempty" header:"EMAIL" binding:"omitempty,email"`
	LogoutURL string `json:"logout-url,omitempty" header:"LOGOUT" binding:"omitempty,uri"`
}

// UserAuthentication is a middleware to fill information about the
// current user. It does not really perform authentication but relies
// on HTTP headers.
func (c *Component) UserAuthentication() gin.HandlerFunc {
	return func(gc *gin.Context) {
		var info UserInformation
		if err := gc.ShouldBindWith(&info, customHeaderBinding{c}); err != nil {
			if c.config.DefaultUser.Login == "" {
				gc.JSON(http.StatusUnauthorized, gin.H{"message": "No user logged in."})
				gc.Abort()
				return
			}
			info = c.config.DefaultUser
		}
		gc.Set("user", info)
		gc.Next()
	}
}

type customHeaderBinding struct {
	c *Component
}

func (customHeaderBinding) Name() string {
	return "header"
}

// Bind will bind struct fields to HTTP headers using the configured mapping.
func (b customHeaderBinding) Bind(req *http.Request, obj interface{}) error {
	value := reflect.ValueOf(obj).Elem()
	tValue := reflect.TypeOf(obj).Elem()
	if value.Kind() != reflect.Struct {
		panic("should be a struct")
	}
	for i := range tValue.NumField() {
		sf := tValue.Field(i)
		if sf.PkgPath != "" && !sf.Anonymous { // unexported
			continue
		}
		tag := sf.Tag.Get("header")
		if tag == "" || tag == "-" {
			continue
		}
		var header string
		switch tag {
		case "LOGIN":
			header = b.c.config.Headers.Login
		case "NAME":
			header = b.c.config.Headers.Name
		case "EMAIL":
			header = b.c.config.Headers.Email
		case "LOGOUT":
			header = b.c.config.Headers.LogoutURL
		}
		if header == "" {
			continue
		}
		value.Field(i).SetString(req.Header.Get(header))
	}

	return binding.Validator.ValidateStruct(obj)
}
