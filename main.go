package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
	"unicode/utf8"

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
	wordMap   map[rune]Word
	wordChan  chan Word
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
	t.wordMap = make(map[rune]Word)
	t.wordChan = make(chan Word)
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
		t.wordChan <- Word{0, rand.Intn(t.colSize), "2test", 2}
		t.wordChan <- Word{0, rand.Intn(t.colSize), "test", 1}
		time.Sleep(800 * time.Millisecond)
	}
}

func (t *TypingGeek) GoMainProcessor() {
	for {
		select {
		case newWord := <-t.wordChan: // receive new word and spawn a new go word routine
			// add newWord to wordPool for rendering
			key, _ := utf8.DecodeRuneInString(newWord.str)
			if _, present := t.wordMap[key]; present {
				continue
			}
			t.wordMap[key] = newWord
			// spawn go routine for each word to process itself (moving)
			go func(key rune) {
				for {
					veloSleepTime := time.Duration(100000/newWord.velo) * time.Microsecond
					time.Sleep(veloSleepTime)
					// due to issue #3117, we gotta assign value like this for map of struct for now.
					tmp := t.wordMap[key]
					tmp.r++
					t.wordMap[key] = tmp
					// delete word that goes out of windows in wordMap
					if t.wordMap[key].r > t.rowSize {
						delete(t.wordMap, key)
						return
					}
				}
			}(key)
		case <-time.After(500 * time.Millisecond):
			//fmt.Println("timeout")
		}
	}
}

func (t *TypingGeek) GoRender() {
	for {
		termbox.Clear(termbox.ColorWhite, termbox.ColorBlack)
		for _, word := range t.wordMap {
			for pos, char := range word.str {
				termbox.SetCell(word.c+pos, word.r, char, termbox.ColorWhite, termbox.ColorBlack)
			}
		}
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
