package web

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/suserc0de/rhui"
)

var globalEngine *rhui.Engine

func SetGlobalRHUIEngine(e *rhui.Engine) {
	globalEngine = e
}

func GetGlobalRHUIEngine() *rhui.Engine {
	return globalEngine
}

func RenderWithLayout(c *gin.Context, status int, layoutName, pageName string, data any) error {
	if globalEngine == nil {
		return fmt.Errorf("global RHUI engine not initialized")
	}
	c.Status(status)
	c.Header("Content-Type", "text/html; charset=utf-8")
	return globalEngine.RenderWithLayout(c.Writer, layoutName, pageName, data)
}
