package web

import (
	"fmt"
	"io"

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

// RenderFragment merender template / komponen HTML parsial (fragment) tanpa layout utuh
// Sangat ideal untuk HTMX, infinite scroll, dan real-time live search DOM injection.
func RenderFragment(c *gin.Context, status int, templateName string, data any) error {
	if globalEngine == nil {
		return fmt.Errorf("global RHUI engine not initialized")
	}
	c.Status(status)
	c.Header("Content-Type", "text/html; charset=utf-8")
	return globalEngine.Render(c.Writer, templateName, data)
}

// RenderFragmentToString merender template parsial ke string (berguna untuk testing atau payload SSE/JSON)
func RenderFragmentToString(templateName string, data any) (string, error) {
	if globalEngine == nil {
		return "", fmt.Errorf("global RHUI engine not initialized")
	}
	return globalEngine.RenderString(templateName, data)
}

// RenderWriter merender template langsung ke io.Writer
func RenderWriter(w io.Writer, templateName string, data any) error {
	if globalEngine == nil {
		return fmt.Errorf("global RHUI engine not initialized")
	}
	return globalEngine.Render(w, templateName, data)
}
