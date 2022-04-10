//go:build release

package http

import "github.com/gin-gonic/gin"

func init() {
	gin.SetMode(gin.ReleaseMode)
}
