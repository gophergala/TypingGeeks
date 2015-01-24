package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/nsf/termbox-go"
)

var mutex = &sync.Mutex{}

type Word struct {
	r, c int
	str  string
	velo int
}

type TypingGeek struct {
	eventChan chan termbox.Event
	exitChan  chan string
	wordPool  []*Word
	wordChan  chan *Word
	rowSize   int
	colSize   int
	fps       int // frame per sec, must be between 1-60, default is 25

}

func (t *TypingGeek) Initialise() {
	// prepare rand engine
	rand.Seed(time.Now().UnixNano())

	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	termbox.Clear(termbox.ColorWhite, termbox.ColorBlack)
	// initialise all go channels
	t.eventChan = make(chan termbox.Event)
	t.exitChan = make(chan string)
	t.wordPool = make([]*Word, 0, 100)
	t.wordChan = make(chan *Word)
	// screen related
	t.rowSize = 30
	t.colSize = 60
	t.fps = 25
}

func (t *TypingGeek) WaitExit() {
	<-t.exitChan
	termbox.Close()
}

func (t *TypingGeek) GoKeyScanner() {
	for {
		event := termbox.PollEvent()
		t.eventChan <- event
	}
}

func (t *TypingGeek) GoWordFeeder() {
	for {
		t.wordChan <- &Word{0, rand.Intn(t.colSize), "test", 1}
		time.Sleep(800 * time.Millisecond)
	}
}

func (t *TypingGeek) GoMainProcessor() {
	for {
		select {
		case newWord := <-t.wordChan: // receive new word and spawn a new go word routine
			mutex.Lock()
			t.wordPool = append(t.wordPool, newWord)
			mutex.Unlock()
			go func() {
				for {
					veloSleepTime := time.Duration(100000/newWord.velo) * time.Microsecond
					time.Sleep(veloSleepTime)
					newWord.r++
					// delete word that goes out of windows in wordPool
					if newWord.r > t.rowSize {
						// TODO: find a way to delete word pointer out with less time complexity
						*newWord = Word{}
						return
					}
				}
			}()
		case <-time.After(500 * time.Millisecond):
			//fmt.Println("timeout")
		}
	}
}

func (t *TypingGeek) GoRender() {
	for {
		termbox.Clear(termbox.ColorWhite, termbox.ColorBlack)
		// lock to make sure nothing change while rendering
		mutex.Lock()
		for idx := 0; idx < len(t.wordPool); {
			// clear out word of the wordPool if find empty value (already expired)
			if *(t.wordPool[idx]) == (Word{}) {
				t.wordPool[idx], t.wordPool[len(t.wordPool)-1], t.wordPool = t.wordPool[len(t.wordPool)-1], nil, t.wordPool[:len(t.wordPool)-1]
			} else {
				for pos, char := range t.wordPool[idx].str {
					termbox.SetCell(t.wordPool[idx].c+pos, t.wordPool[idx].r, char, termbox.ColorWhite, termbox.ColorBlack)
				}
				idx++
			}
		}
		mutex.Unlock()
		termbox.Flush()
		fpsSleepTime := time.Duration(1000000/t.fps) * time.Microsecond
		time.Sleep(fpsSleepTime)
	}

}

func (t *TypingGeek) GoKeyAnalyzer() {
	for {
		select {
		case event := <-t.eventChan:
			switch event.Type {
			case termbox.EventKey:
				switch event.Key {
				case termbox.KeyCtrlC, termbox.KeyCtrlX:
					fmt.Println("try to exit")
					t.exitChan <- "end"
					//return
				}
				fmt.Println(string(event.Ch))
			case termbox.EventResize:
				fmt.Println("Resize")
			case termbox.EventError:
				panic(event.Err)
			}
		}
	}
}

func main() {
	t := new(TypingGeek)
	t.Initialise()
	go t.GoKeyScanner()
	go t.GoKeyAnalyzer()
	go t.GoRender()
	go t.GoMainProcessor()
	go t.GoWordFeeder()
	t.WaitExit()
}
