package sse

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/gofiber/fiber/v2"
)

// EventMessage merepresentasikan struktur data yang akan dikirim ke browser
type EventMessage struct {
	Topic string      // Target penerima. Misal: "monitor_tv" atau ID Tiket "uuid-1234"
	Event string      // Nama event HTMX. Misal: "monitor_active" atau "status_updated"
	Data  interface{} // Data tambahan (Opsional, akan di-convert ke JSON)
}

// Client merepresentasikan satu koneksi browser yang sedang terbuka (TV / HP)
type Client struct {
	Channel chan EventMessage
	Topic   string
}

// Manager adalah struktur utama yang mengelola semua koneksi SSE yang masuk
type Manager struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan EventMessage
	mu         sync.RWMutex
}

// NewManager membuat instance SSE Manager baru dan menjalankan Hub Goroutine
func NewManager() *Manager {
	manager := &Manager{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan EventMessage),
	}

	// Menjalankan hub secara asinkron (background)
	go manager.run()

	return manager
}

// run adalah infinite loop yang memproses penambahan, penghapusan, dan siaran pesan
func (m *Manager) run() {
	for {
		select {
		// Ketika ada browser baru yang terhubung
		case client := <-m.register:
			m.mu.Lock()
			m.clients[client] = true
			m.mu.Unlock()
			log.Printf("🔌 SSE Klien Baru Terhubung (Topik: %s). Total Klien: %d\n", client.Topic, len(m.clients))

		// Ketika browser ditutup / disconnect
		case client := <-m.unregister:
			m.mu.Lock()
			if _, ok := m.clients[client]; ok {
				delete(m.clients, client)
				close(client.Channel)
				log.Printf("🔌 SSE Klien Terputus (Topik: %s). Total Klien: %d\n", client.Topic, len(m.clients))
			}
			m.mu.Unlock()

		// Ketika petugas menekan tombol Panggil (Pesan masuk ke broadcast)
		case message := <-m.broadcast:
			m.mu.RLock()
			for client := range m.clients {
				// Cek apakah topik klien cocok dengan tujuan pesan
				// (Topik "all" berarti kirim ke semua orang)
				if client.Topic == message.Topic || message.Topic == "all" {
					// Gunakan non-blocking send (select dengan default)
					// agar jika satu klien nge-lag/error, klien lain tidak ikut terhenti
					select {
					case client.Channel <- message:
					default:
						// Jika channel penuh, anggap klien mati secara paksa
						delete(m.clients, client)
						close(client.Channel)
					}
				}
			}
			m.mu.RUnlock()
		}
	}
}

// Broadcast adalah fungsi bantuan untuk dipanggil dari Handler/Service
// guna menyiarkan pesan ke topik tertentu
func (m *Manager) Broadcast(topic, event string, data interface{}) {
	m.broadcast <- EventMessage{
		Topic: topic,
		Event: event,
		Data:  data,
	}
}

// HandleSSE menghasilkan Fiber Handler untuk menerima koneksi HTTP dan mengubahnya menjadi stream SSE
func (m *Manager) HandleSSE(topic string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Set Header khusus untuk SSE agar koneksi tidak diputus oleh browser
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")

		// 2. SetBodyStreamWriter memungkinkan kita menulis balasan ke browser terus-menerus
		c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
			// Buat representasi klien dengan channel (buffer 10 pesan)
			client := &Client{
				Channel: make(chan EventMessage, 10),
				Topic:   topic,
			}

			// Daftarkan klien ini ke Manager
			m.register <- client

			// Defer memastikan jika user menutup tab HP, klien akan dihapus dari memori server
			defer func() {
				m.unregister <- client
			}()

			// Loop untuk mendengarkan pesan masuk ke channel klien ini
			for msg := range client.Channel {
				// Konversi data opsional ke JSON String
				dataBytes, err := json.Marshal(msg.Data)
				if err != nil {
					log.Printf("SSE JSON Error: %v\n", err)
					continue
				}

				// Format penulisan SSE wajib:
				// event: nama_event
				// data: isi_data\n\n
				fmt.Fprintf(w, "event: %s\n", msg.Event)
				fmt.Fprintf(w, "data: %s\n\n", string(dataBytes))

				// Push/Flush data secara paksa langsung ke browser (tidak di-buffer)
				err = w.Flush()
				if err != nil {
					// Jika gagal flush, berarti koneksi TCP sudah terputus dari pihak browser
					log.Printf("Koneksi TCP browser terputus\n")
					return
				}
			}
		})

		return nil
	}
}
