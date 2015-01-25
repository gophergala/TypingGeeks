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

var wordList = []string{"cat", "card", "cog", "cam", "camfrog", "caddie", "cammy"}

//var wordList = []string{"cat", "dog", "max", "delicious", "games", "ant", "min"}

type Word struct {
	r, c         int
	str          string
	velo         int
	progress     int
	typedKeyChan chan rune
}

type TypingGeeks struct {
	eventChan    chan termbox.Event
	exitChan     chan string
	wordMap      map[rune]Word
	wordChan     chan Word
	typedKeyChan chan rune
	curWordKey   rune
	rowSize      int
	colSize      int
	fps          int // frame per sec, must be between 1-60, default is 25

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
	t.typedKeyChan = make(chan rune)
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
		t.wordChan <- Word{
			r:            0,
			c:            rand.Intn(t.colSize),
			str:          wordList[counter],
			velo:         1,
			progress:     0,
			typedKeyChan: make(chan rune),
		}
		time.Sleep(500 * time.Millisecond)
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
					curWord, exist := t.wordMap[key]
					if !exist {
						return
					}
					curWord.r++
					t.wordMap[key] = curWord
					// delete word that goes out of windows in wordMap
					// TODO: need to watch out of race condition for map, too. see -> https://blog.golang.org/go-maps-in-action
					if t.wordMap[key].r > t.rowSize {
						delete(t.wordMap, key)
						return
					}
				}
			}(key)
			go func(key rune) {
				for {
					typedKey := <-t.wordMap[key].typedKeyChan
					curWord, exist := t.wordMap[key]
					if !exist {
						return
					}
					for pos, char := range curWord.str {
						if pos == curWord.progress {
							if char == typedKey {
								curWord.progress++
								if curWord.progress >= len(curWord.str) {
									// TODO: finish whole word, implement successful attempt effect
									delete(t.wordMap, t.curWordKey)
									t.curWordKey = 0
									return
								}
								t.wordMap[t.curWordKey] = curWord
								break
							} else {
								// TODO: wrong key, implement fail attempt effect
							}
						}
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
		case typedKey := <-t.typedKeyChan:
			if t.curWordKey != 0 {
				curWord := t.wordMap[t.curWordKey]
				curWord.typedKeyChan <- typedKey
			} else {
				// due to issue #3117, we gotta assign value like this for map of struct for now.
				if curWord, exist := t.wordMap[typedKey]; exist {
					t.curWordKey = typedKey
					curWord.typedKeyChan <- typedKey
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
			t.typedKeyChan <- event.Ch
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
