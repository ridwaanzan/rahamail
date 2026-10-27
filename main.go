package main

import (
	"log"
	"net/http"
	"os"

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

	// 4. Jalankan Core SMTP Server di background goroutine
	smtpPort := os.Getenv("SMTP_PORT")
	if smtpPort == "" {
		smtpPort = ":2525" // Port non-root untuk testing lokal (port 25 membutuhkan akses root)
	}
	smtpServer := smtpserver.StartSMTPServer(smtpPort, "mail.example.com")
	defer smtpServer.Close()

	// 5. Jalankan IMAP Server di background goroutine
	imapPort := os.Getenv("IMAP_PORT")
	if imapPort == "" {
		imapPort = ":1143" // Port non-root untuk testing lokal (port 143/993 membutuhkan akses root)
	}
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

	// Public Auth & Webmail Routes
	r.GET("/register", authCtrl.RenderRegister)
	r.GET("/login", authCtrl.RenderLogin)
	r.GET("/logout", authCtrl.Logout)

	// User Webmail (Protected by Session Auth Middleware)
	webmail := r.Group("/mail", middleware.UserAuthMiddleware())
	{
		webmail.GET("", mailCtrl.RenderWebmail)
	}

	// Route Home: Merender template HTML (.tmpl) untuk Admin
	r.GET("/", mailCtrl.RenderIndex)
	r.GET("/setup", mailCtrl.RenderSetup)

	// 5. REST API & SSE Routes
	api := r.Group("/api/v1")
	{
		// Public Auth API
		authAPI := api.Group("/auth")
		{
			authAPI.GET("/check-username", authCtrl.CheckUsername)
			authAPI.POST("/register", authCtrl.Register)
			authAPI.POST("/login", authCtrl.Login)
			authAPI.POST("/logout", authCtrl.Logout)
		}

		// User Webmail API (Protected)
		mailAPI := api.Group("/webmail", middleware.UserAuthMiddleware())
		{
			mailAPI.GET("/messages/:id", mailCtrl.GetMessageDetail)
			mailAPI.POST("/send", mailCtrl.ComposeAndSendEmail)
			mailAPI.POST("/schedule", mailCtrl.ScheduleEmail)
			mailAPI.POST("/schedule/:id/cancel", mailCtrl.CancelScheduledMail)
			mailAPI.POST("/messages/move", mailCtrl.MoveMessage)
			mailAPI.POST("/messages/:id/star", mailCtrl.ToggleStarMessage)
			mailAPI.DELETE("/messages/:id", mailCtrl.DeleteMessage)
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

		// Domain Management
		api.GET("/domains", mailCtrl.ListDomains)
		api.POST("/domains", mailCtrl.CreateDomain)

		// User/Mailbox Management
		api.GET("/users", mailCtrl.ListUsers)
		api.POST("/users", mailCtrl.CreateUser)

		// Messages / Webmail
		api.GET("/messages", mailCtrl.ListMessages)
		api.POST("/email/send", mailCtrl.SendEmail)
	}

	webPort := os.Getenv("PORT")
	if webPort == "" {
		webPort = ":8080"
	}
	log.Printf("🚀 RaHaMail Server berjalan: Web UI di http://localhost%s | SMTP di %s | IMAP di %s", webPort, smtpPort, imapPort)
	if err := r.Run(webPort); err != nil {
		log.Fatalf("Gagal menjalankan web server: %v", err)
	}
}
