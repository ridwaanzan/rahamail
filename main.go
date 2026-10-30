package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"mail-server/internal/api/controllers"
	"mail-server/internal/api/middleware"
	"mail-server/internal/imapserver"
	"mail-server/internal/queue"
	"mail-server/internal/repository"
	"mail-server/internal/scheduler"
	"mail-server/internal/smtpserver"
	"mail-server/internal/sse"
	"mail-server/web"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Inisialisasi Database (PostgreSQL / SQLite fallback)
	repository.InitDB()

	// 2. Jalankan SSE Hub di background goroutine
	go sse.GlobalHub.Run()

	// 3. Inisialisasi Asynq Queue Manager & Scheduler Engine
	queue.InitQueue()
	scheduler.InitScheduler()

	// Helper untuk menormalisasi format port (misal "25" -> ":25", ":25" -> ":25")
	normalizePort := func(envVal, defaultPort string) string {
		p := strings.TrimSpace(os.Getenv(envVal))
		if p == "" {
			return defaultPort
		}
		if !strings.HasPrefix(p, ":") && !strings.Contains(p, ":") {
			return ":" + p
		}
		return p
	}

	// 4. Jalankan Core SMTP Server di background goroutine
	smtpPort := normalizePort("SMTP_PORT", ":2525")
	smtpServer := smtpserver.StartSMTPServer(smtpPort, "mail.example.com")
	defer smtpServer.Close()

	// 5. Jalankan IMAP Server di background goroutine
	imapPort := normalizePort("IMAP_PORT", ":1143")
	imapServer := imapserver.StartIMAPServer(imapPort)
	defer imapServer.Close()

	// 6. Setup Gin Web Server
	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = gin.ReleaseMode
	}
	gin.SetMode(ginMode)
	r := gin.Default()

	// Melayani Static Assets langsung dari Memory Binary (go:embed)
	staticFS, err := web.GetStaticFS()
	if err != nil {
		log.Fatalf("❌ Gagal memuat embedded static filesystem: %v", err)
	}
	r.StaticFS("/static", staticFS)

	// Memuat Template HTML (.tmpl) langsung dari Memory Binary (go:embed)
	tmpl, err := web.GetTemplates()
	if err != nil {
		log.Fatalf("❌ Gagal memuat embedded templates: %v", err)
	}
	r.SetHTMLTemplate(tmpl)

	// Controller
	mailCtrl := controllers.NewMailController()
	authCtrl := controllers.NewAuthController()
	simCtrl := controllers.NewSimulationController()

	// Public Auth & Webmail Routes
	r.GET("/register", authCtrl.RenderRegister)
	r.GET("/login", authCtrl.RenderLogin)
	r.GET("/logout", authCtrl.Logout)

	// Phishing & Spam Awareness Simulation Public Tracking Route
	r.GET("/sim/c/:code", simCtrl.HandleTrackingClick)

	// User Webmail (Protected by Session Auth Middleware)
	webmail := r.Group("/mail", middleware.UserAuthMiddleware())
	{
		webmail.GET("", mailCtrl.RenderWebmail)
	}

	// Admin Multi-Page Views (Protected or Open for Local Admin Dashboard)
	r.GET("/", mailCtrl.RenderIndex)
	r.GET("/admin/domains", mailCtrl.RenderAdminDomains)
	r.GET("/admin/simulations", mailCtrl.RenderAdminSimulations)
	r.GET("/admin/logs", mailCtrl.RenderAdminLogs)
	r.GET("/admin/settings", mailCtrl.RenderAdminSettings)
	r.GET("/setup", mailCtrl.RenderSetup)

	// 5. REST API & SSE Routes
	api := r.Group("/api/v1")
	{
		// Public Auth API (Dilindungi Token Bucket Rate Limiter)
		authAPI := api.Group("/auth")
		{
			authAPI.GET("/check-username", authCtrl.CheckUsername)
			authAPI.POST("/register", middleware.RateLimitMiddleware(middleware.RegisterLimiter, "Batas pendaftaran tercapai. Silakan coba lagi nanti."), authCtrl.Register)
			authAPI.POST("/login", middleware.RateLimitMiddleware(middleware.AuthLimiter, "Terlalu banyak percobaan masuk. Silakan tunggu 1 menit."), authCtrl.Login)
			authAPI.POST("/verify-2fa", middleware.RateLimitMiddleware(middleware.AuthLimiter, "Terlalu banyak percobaan verifikasi 2FA."), authCtrl.Verify2FA)
			authAPI.POST("/logout", authCtrl.Logout)
		}

		// 2FA Management API (Protected by User Session)
		twoFA := api.Group("/2fa", middleware.UserAuthMiddleware())
		{
			twoFA.GET("/setup", authCtrl.Setup2FA)
			twoFA.POST("/enable", authCtrl.Enable2FA)
			twoFA.POST("/disable", authCtrl.Disable2FA)
		}

		// Developer API Keys Management (Protected by User Session)
		apiKeys := api.Group("/api-keys", middleware.UserAuthMiddleware())
		{
			apiKeys.GET("", authCtrl.ListAPIKeys)
			apiKeys.POST("", authCtrl.CreateAPIKey)
			apiKeys.DELETE("/:id", authCtrl.RevokeAPIKey)
		}

		// Developer Transactional Sending API (Protected by Bearer rhm_live_... API Key)
		api.POST("/send", middleware.APIKeyAuthMiddleware(), mailCtrl.SendTransactionalMail)

		// User Webmail API (Protected & Outbound Throttled)
		mailAPI := api.Group("/webmail", middleware.UserAuthMiddleware())
		{
			mailAPI.GET("/messages/:id", mailCtrl.GetMessageDetail)
			mailAPI.POST("/send", middleware.UserRateLimitMiddleware(middleware.OutboundSendLimiter, "Batas pengiriman pesan terlampaui."), mailCtrl.ComposeAndSendEmail)
			mailAPI.POST("/schedule", mailCtrl.ScheduleEmail)
			mailAPI.POST("/schedule/:id/cancel", mailCtrl.CancelScheduledMail)
			mailAPI.POST("/messages/move", mailCtrl.MoveMessage)
			mailAPI.POST("/messages/:id/star", mailCtrl.ToggleStarMessage)
			mailAPI.DELETE("/messages/:id", mailCtrl.DeleteMessage)
			mailAPI.POST("/trash/empty", mailCtrl.EmptyTrash)

			// Contacts / Address Book API
			mailAPI.GET("/contacts", mailCtrl.ListContacts)
			mailAPI.POST("/contacts", mailCtrl.CreateContact)
			mailAPI.DELETE("/contacts/:id", mailCtrl.DeleteContact)

			// Out-of-Office / Vacation Responder API
			mailAPI.GET("/auto-responder", mailCtrl.GetAutoResponder)
			mailAPI.POST("/auto-responder", mailCtrl.SaveAutoResponder)

			// Canned Responses / Templates API
			mailAPI.GET("/templates", mailCtrl.ListCannedResponses)
			mailAPI.POST("/templates", mailCtrl.CreateCannedResponse)
			mailAPI.DELETE("/templates/:id", mailCtrl.DeleteCannedResponse)
		}

		// SSE live stream endpoint
		api.GET("/sse", sse.GlobalHub.SSEHandler)

		api.GET("/healthz", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status": "healthy",
				"services": gin.H{
					"gin_api":     "online",
					"database":    "connected",
					"sse_hub":     "active",
					"smtp_server": "active (listening on " + smtpPort + ")",
				},
			})
		})

		api.GET("/stats", mailCtrl.GetStats)

		// Setup Wizard API
		api.POST("/setup/complete", mailCtrl.CompleteSetup)

		// Cloud Backup & S3 API (Super Admin)
		api.GET("/admin/backup/config", mailCtrl.GetBackupConfig)
		api.POST("/admin/backup/config", mailCtrl.SaveBackupConfig)
		api.POST("/admin/backup/trigger", mailCtrl.TriggerManualBackup)

		// Phishing & Spam Awareness Simulation Management (Admin)
		api.GET("/admin/simulations/presets", simCtrl.ListPresets)
		api.GET("/admin/simulations", simCtrl.ListCampaigns)
		api.POST("/admin/simulations", simCtrl.CreateCampaign)
		api.POST("/admin/simulations/:id/launch", simCtrl.LaunchCampaign)
		api.GET("/admin/simulations/:id/stats", simCtrl.GetCampaignStats)

		// Domain Management
		api.GET("/domains", mailCtrl.ListDomains)
		api.POST("/domains", mailCtrl.CreateDomain)
		api.POST("/domains/:id/catch-all", mailCtrl.SetCatchAll)

		// User/Mailbox Management
		api.GET("/users", mailCtrl.ListUsers)
		api.POST("/users", mailCtrl.CreateUser)

		// Messages / Webmail
		api.GET("/messages", mailCtrl.ListMessages)
		api.POST("/email/send", mailCtrl.SendEmail)
	}

	webPort := normalizePort("PORT", ":9000")
	log.Printf("🚀 RaHaMail Server berjalan: Web UI di http://localhost%s | SMTP di %s | IMAP di %s", webPort, smtpPort, imapPort)
	if err := r.Run(webPort); err != nil {
		log.Fatalf("Gagal menjalankan web server: %v", err)
	}
}
