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

var wordList = []string{"cat", "dog", "max", "delicious", "games", "ant", "min"}

type Word struct {
	r, c     int
	str      string
	velo     int
	progress int
}

type TypingGeeks struct {
	eventChan  chan termbox.Event
	exitChan   chan string
	wordMap    map[rune]Word
	wordChan   chan Word
	keyChan    chan rune
	curWordKey rune
	rowSize    int
	colSize    int
	fps        int // frame per sec, must be between 1-60, default is 25

}

func (t *TypingGeeks) Initialise() {
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
	t.keyChan = make(chan rune)
	// screen related
	t.rowSize = 30
	t.colSize = 60
	t.fps = 25
}

func (t *TypingGeeks) WaitExit() {
	<-t.exitChan
	termbox.Close()
}

func (t *TypingGeeks) GoWordFeeder() {
	counter := 0
	for {
		t.wordChan <- Word{0, rand.Intn(t.colSize), wordList[counter], 2, 0}
		time.Sleep(800 * time.Millisecond)
		counter++
		if counter >= len(wordList) {
			counter = 0
		}
	}
}

func (t *TypingGeeks) GoMainProcessor() {
	for {
		select {
		case newWord := <-t.wordChan: // receive new word and spawn a new go word routine
			// add newWord to wordPool for rendering
			key, _ := utf8.DecodeRuneInString(newWord.str)
			if _, exist := t.wordMap[key]; exist {
				continue
			}
			t.wordMap[key] = newWord
			// spawn go routine for each word to process itself (moving)
			go func(key rune) {
				for {
					veloSleepTime := time.Duration(1000000/newWord.velo) * time.Microsecond
					time.Sleep(veloSleepTime)
					// due to issue #3117, we gotta assign value like this for map of struct for now.
					tmp := t.wordMap[key]
					tmp.r++
					t.wordMap[key] = tmp
					// delete word that goes out of windows in wordMap
					// TODO: need to watch out of race condition for map, too. see -> https://blog.golang.org/go-maps-in-action
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

func (t *TypingGeeks) GoRender() {
	for {
		termbox.Clear(termbox.ColorWhite, termbox.ColorBlack)
		for _, word := range t.wordMap {
			for pos, char := range word.str {
				if pos >= word.progress {
					termbox.SetCell(word.c+pos, word.r, char, termbox.ColorWhite, termbox.ColorBlack)
				}
			}
		}
		termbox.Flush()
		fpsSleepTime := time.Duration(1000000/t.fps) * time.Microsecond
		time.Sleep(fpsSleepTime)
	}

}

func (t *TypingGeeks) GoKeyAnalyzer() {
	for {
		select {
		case key := <-t.keyChan:
			if t.curWordKey != 0 {

			} else {
				// due to issue #3117, we gotta assign value like this for map of struct for now.
				if tmp, exist := t.wordMap[key]; exist {
					t.curWordKey = key
					tmp.progress++
					t.wordMap[key] = tmp
				}
			}
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (t *TypingGeeks) GoEventTrigger() {
	for {
		event := termbox.PollEvent()
		switch event.Type {
		case termbox.EventKey:
			switch event.Key {
			case termbox.KeyCtrlC, termbox.KeyCtrlX:
				// exit
				t.exitChan <- "end"
			case termbox.KeyEsc:
				// esc to cancel typing current word
				if t.curWordKey != 0 {
					tmp := t.wordMap[t.curWordKey]
					tmp.progress = 0
					t.wordMap[t.curWordKey] = tmp
					t.curWordKey = 0
				}
			}
			// TODO: this is blocking if you type fast, maybe use select? or use slice of channel?
			t.keyChan <- event.Ch
		case termbox.EventResize:
			fmt.Println("Resize")
		case termbox.EventError:
			panic(event.Err)
		}
	}
}

func main() {
	t := new(TypingGeeks)
	t.Initialise()
	go t.GoEventTrigger()
	go t.GoKeyAnalyzer()
	go t.GoRender()
	go t.GoMainProcessor()
	go t.GoWordFeeder()
	t.WaitExit()
}
