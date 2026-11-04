---
name: rhui-component-development
description: >-
  Expert guidance for developing, testing, and debugging server-rendered web applications 
  using RHUI (Go Component Template Engine). Covers template syntax (.rhui), components, 
  props, default and named slots, attribute spreading (:attrs), control flow directives (<if>, <each>), 
  built-in pipe filters (| rupiah, | upper), framework adapters (Gin, Echo, Fiber, Chi, net/http), 
  realtime SSE streaming with HTMX, and SEO sitemap/JSON-LD generation.
---

# RHUI AI Skill: Component-Based SSR Template Engine for Go

This skill guides AI agents and developers in building high-performance, secure, and reactive server-side rendered (SSR) web applications using **RHUI** (`github.com/suserc0de/rhui`).

---

## 1. Core Architecture & Mental Model

RHUI compiles declarative HTML-like component templates (`.rhui`) into pre-parsed immutable Abstract Syntax Trees (AST). At runtime:
- **Compiled Expression AST**: Dynamic expressions (`{{ ... }}`, `:attr="..."`, `<if test="...">`) are compiled into AST nodes with thread-safe caching and pooled evaluation contexts (`sync.Pool`).
- **Zero-Allocation Fast-Path Escaping**: Context-aware escaping scans for sensitive characters (`&<>"'`) before copying; plain text returns 0 heap allocations.
- **Direct Writer Streaming**: Serializes elements via direct `io.WriteString` and `strconv` without `fmt.Fprintf` reflection bottlenecks.
- **Sub-Scope Recycling**: Loop and component scopes are recycled via `sync.Pool`, eliminating garbage collector pressure on large tables.
- **No Node.js / JavaScript Build Step**: Pure Go runtime execution. Reactivity is achieved seamlessly via HTMX fragments and Server-Sent Events (SSE).

---

## 2. Quickstart & Engine Initialization

### Package Import & Initialization
Always initialize the engine using either functional options `rhui.New()` or the convenience constructor `rhui.NewEngine()`:

```go
package main

import (
    "log"
    "net/http"
    "github.com/suserc0de/rhui"
)

func main() {
    // Standard initialization with embedded or disk directory
    engine, err := rhui.New(
        rhui.WithDir("views"),
        rhui.WithExtension(".rhui"),
    )
    if err != nil {
        log.Fatalf("failed to initialize RHUI engine: %v", err)
    }

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        data := map[string]any{
            "Title": "RHUI Application",
            "User":  map[string]any{"Name": "Alex", "IsAdmin": true},
        }
        _ = engine.Render(w, "pages/home", data)
    })

    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

---

## 3. Component Authoring Guidelines

### File Naming & Location
- Place reusable components in `views/components/<ComponentName>.rhui`.
- Component names must follow PascalCase (e.g., `Card.rhui`, `Button.rhui`, `Modal.rhui`).

### Component Definition (`<component>`)
A component definition wraps encapsulated markup, declares props, and projects slots:

```html
<!-- views/components/Card.rhui -->
<component name="Card">
  <props title="Default Title" badge="" />

  <div class="card border rounded-xl p-4 bg-slate-900 border-slate-800">
    <div class="card-header flex justify-between items-center mb-2">
      <h3 class="font-bold text-white">{{ title | upper }}</h3>
      <if test="badge != ''">
        <span class="badge text-xs px-2 py-0.5 rounded bg-brand-500/20 text-brand-300">{{ badge }}</span>
      </if>
    </div>

    <!-- Default Slot with Fallback Content -->
    <div class="card-body text-slate-300">
      <slot>No content provided.</slot>
    </div>

    <!-- Named Slot Projection -->
    <if test="has_slot('footer')">
      <div class="card-footer mt-4 pt-2 border-t border-slate-800 text-xs text-slate-400">
        <slot:footer />
      </div>
    </if>
  </div>
</component>
```

### Invoking Components (`<ui:ComponentName>`)
Invoke components inside pages or layouts using `<ui:Name>`:

```html
<!-- views/pages/home.rhui -->
<ui:Card title="Analytics Overview" badge="Live">
  <p>System traffic increased by 24% this week.</p>

  <!-- Content projected into named slot 'footer' -->
  <slot:footer>
    <a href="/metrics" class="text-brand-400 hover:underline">View detailed metrics &rarr;</a>
  </slot:footer>
</ui:Card>
```

### Attribute Spreading (`:attrs`)
Forward arbitrary HTML attributes (e.g., `hx-post`, `aria-*`, `class`) directly to HTML tags:

```html
<!-- views/components/Input.rhui -->
<component name="Input">
  <props label="" />

  <div class="form-group">
    <if test="label != ''">
      <label class="block text-xs font-semibold mb-1">{{ label }}</label>
    </if>
    <input class="px-3 py-2 rounded bg-slate-900 border border-slate-700 w-full" :attrs />
  </div>
</component>
```

---

## 4. Syntax & Template Directives

### Expressions & Interpolation
Interpolate values safely using `{{ expression }}`:
```html
<p>User: {{ User.Name }}</p>
<p>Active: {{ User.IsActive ? 'Yes' : 'No' }}</p>
```

### Conditionals (`<if>` & `<else>`)
```html
<if test="User.IsAdmin == true">
  <span class="badge-admin">Administrator</span>
</if>
<else>
  <span class="badge-user">Member</span>
</else>
```

### Loops with Position Metadata (`<each>`)
Iterate slices or arrays with automatic loop metadata injection (`@first`, `@last`, `@index`, `@even`, `@odd`):
```html
<each items="Orders" as="order">
  <tr class="{{ @even ? 'bg-slate-800/40' : '' }}">
    <td>{{ @index + 1 }}</td>
    <td>{{ order.Customer }}</td>
    <td>{{ order.Amount | rupiah }}</td>
    <td>
      <if test="@first">
        <span class="badge">Latest Order</span>
      </if>
    </td>
  </tr>
</each>
```

### Built-in Pipe Filters
Chain transformations cleanly inside interpolations:
- `| upper` : Uppercase string (`{{ Title | upper }}`)
- `| lower` : Lowercase string (`{{ Email | lower }}`)
- `| rupiah` : Currency formatting for IDR (`{{ Price | rupiah }}` &rarr; `Rp 150.000`)
- `| dateFormat('2006-01-02')` : Go time formatting

---

## 5. Realtime SSE Streaming & Live Components

Stream updated HTML component diffs over standard HTTP/2 Server-Sent Events to HTMX:

### 1. Go Broker Setup
```go
import "github.com/suserc0de/rhui/sse"

broker := sse.NewBroker()

// SSE stream endpoint
http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
    stream, err := broker.Connect(w, r)
    if err != nil {
        http.Error(w, "SSE failed", http.StatusBadRequest)
        return
    }
    defer stream.Close()
    stream.Wait()
})

// Trigger live component broadcast
func onNewOrder(engine *rhui.Engine, order Order) {
    _ = broker.BroadcastComponent(engine, "components/OrderCard", map[string]any{
        "Order": order,
    })
}
```

### 2. Client HTMX SSE Consumer
```html
<div hx-ext="sse" sse-connect="/events">
  <h2>Live Order Feed</h2>
  <div sse-swap="OrderCard" hx-swap="afterbegin" id="order-feed">
    <!-- Streamed component HTML is dynamically prepended here -->
  </div>
</div>
```

---

## 6. Framework Adapters Cookbook

Connect RHUI into any popular Go router in one line:

### Gin
```go
import (
    "github.com/gin-gonic/gin"
    "github.com/suserc0de/rhui"
    rhuigin "github.com/suserc0de/rhui/adapter/gin"
)

r := gin.Default()
r.HTMLRender = rhuigin.New(engine)

r.GET("/", func(c *gin.Context) {
    c.HTML(200, "pages/home", gin.H{"Title": "Gin with RHUI"})
})
```

### Echo
```go
import (
    "github.com/labstack/echo/v4"
    "github.com/suserc0de/rhui"
    rhuiecho "github.com/suserc0de/rhui/adapter/echo"
)

e := echo.New()
e.Renderer = rhuiecho.New(engine)

e.GET("/", func(c echo.Context) error {
    return c.Render(200, "pages/home", echo.Map{"Title": "Echo with RHUI"})
})
```

### Fiber
```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/suserc0de/rhui"
    rhuifiber "github.com/suserc0de/rhui/adapter/fiber"
)

app := fiber.New(fiber.Config{
    Views: rhuifiber.New(engine),
})

app.Get("/", func(c *fiber.Ctx) error {
    return c.Render("pages/home", fiber.Map{"Title": "Fiber with RHUI"})
})
```

### Chi & Standard net/http
```go
import (
    "github.com/go-chi/chi/v5"
    "github.com/suserc0de/rhui"
    rhuichi "github.com/suserc0de/rhui/adapter/chi"
)

adapter := rhuichi.New(engine)
r := chi.NewRouter()

r.Get("/", func(w http.ResponseWriter, r *http.Request) {
    adapter.Render(w, "pages/home", map[string]any{"Title": "Chi with RHUI"})
})
```

---

## 7. Built-in SEO & Sitemap Utilities

Generate Google-compliant XML sitemaps and Schema.org JSON-LD structured data:

```go
import "github.com/suserc0de/rhui/seo"

func serveSitemap(w http.ResponseWriter, r *http.Request) {
    sm := seo.NewSitemap()
    sm.Add(seo.SitemapURL{
        Loc:        "https://example.com/",
        LastMod:    time.Now().Format("2006-01-02"),
        ChangeFreq: seo.Daily,
        Priority:   1.0,
    })

    xmlBytes, _ := sm.XML()
    w.Header().Set("Content-Type", "application/xml; charset=utf-8")
    w.Write(xmlBytes)
}
```

---

## 8. Best Practices & Troubleshooting

- **Never concatenate raw HTML in Go handlers**: Keep UI layout and tag composition inside `.rhui` components; pass clean domain structs or maps from Go.
- **Always close `<if>` before opening `<else>`**: In RHUI syntax, write `<if test="...">...</if><else>...</else>`.
- **VS Code Syntax Highlighting**: Add `"files.associations": { "*.rhui": "html" }` in `.vscode/settings.json`.
- **Concurrency & Thread Safety**: All template ASTs are immutable and safe for concurrent reads across goroutines (`go test -race ./...` clean).
