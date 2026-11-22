package main

import (
	"flag"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
	"strconv"

	"github.com/gin-gonic/gin"
)

// e4 metrics. how many booking succeeded vs rejected
type Metrics struct {
	booked	atomic.Int64
	rejected	atomic.Int64
}

func main() {
	// how many requests the server can work on at the same time
	capacity := flag.Int("capacity", 5, "max requests handled at once")

	// how long each booking takes to process
	work := flag.Duration("work", 100*time.Millisecond, "time to process one booking")

	// waiting room flags
	waitingRoom := flag.Bool("waiting-room", false, "enable the waiting room")
	admitPerTick := flag.Int("admit-per-tick", 5, "tickets admitted every 100ms")
	flag.Parse()

	fmt.Println("capacity:", *capacity, "work:", *work)

	// buffered channel with "capacity" spots. a full channel means the server is busy
	spots := make(chan struct{}, *capacity)
	metrics := &Metrics{}

	wr := NewWaitingRoom()
	if *waitingRoom {
		go wr.Run(*admitPerTick, 100*time.Millisecond)
	}
	fmt.Println("waiting room:", *waitingRoom)

	// router := gin.Default()
	router := gin.New()
	router.Use(gin.Recovery())

	router.POST("/book", func(c *gin.Context) {
		select {
		case spots <- struct{}{}:
			// got a spot. free it when request finishes
			defer func() { <- spots }()

			// simulate booking process time
			time.Sleep(*work)
			metrics.booked.Add(1)
			c.JSON(http.StatusOK, gin.H{"status": "booked"})

		default: 
			// all spots taken. the server then is overloaded
			metrics.rejected.Add(1)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "server busy"})
		}
	})

	router.POST("/join", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ticket": wr.Join()})
	})

	router.GET("/status", func(c *gin.Context) {
		// reads ticket from URL as text
		id, err := strconv.ParseInt(c.Query("ticket"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad ticket"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"admitted": wr.IsAdmitted(id)})
	})

	router.GET("/metrics", func(c *gin.Context) {
		inLine, admitted, avgWait, maxWait := wr.Stats()
		c.JSON(http.StatusOK, gin.H{
			"booked": 	metrics.booked.Load(),
			"rejected": 	metrics.rejected.Load(),
			"in_line":	inLine,
			"admitted": admitted,
			"avg_wait_ms": avgWait, 
			"max_wait_ms": maxWait,
		})
	})

	router.Run(":8081")
}