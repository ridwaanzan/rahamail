package controllers

import (
	"net/http"

	"mail-server/internal/simulation"

	"github.com/gin-gonic/gin"
)

type SimulationController struct{}

func NewSimulationController() *SimulationController {
	return &SimulationController{}
}

// HandleTrackingClick menangani klik tautan tracking simulasi: GET /sim/c/:code
func (sc *SimulationController) HandleTrackingClick(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.String(http.StatusBadRequest, "Invalid tracking code")
		return
	}

	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	target, err := simulation.RecordClick(code, ipAddress, userAgent)
	if err != nil {
		c.String(http.StatusNotFound, "Tautan simulasi tidak valid atau telah kedaluwarsa.")
		return
	}

	senderEmail := "unknown@suspicious-domain.xyz"
	if target.Campaign != nil && target.Campaign.SenderEmail != "" {
		senderEmail = target.Campaign.SenderEmail
	}

	// Render halaman Teachable Moment (Edukasi Keamanan)
	c.HTML(http.StatusOK, "simulation_teachable.tmpl", gin.H{
		"title":        "Peringatan Edukasi Keamanan",
		"sender_email": senderEmail,
		"campaign":     target.Campaign,
		"target":       target,
	})
}
