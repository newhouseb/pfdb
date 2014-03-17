package main

import (
	"bufio"
	"fmt"
	"github.com/nsf/termbox-go"
	"os"
	"strings"
	"time"
)

type ProgramState struct {
	offset               int
	buffer               []string
	timestamps           []int64
	log                  map[string]*HistoricalVariable
	focused_pane         int
	selected_index       int
	selected_buffer_line int
	timecursor           int64
	realtime             bool
	start_time           int64
}

type HistoricalVariable struct {
	Name      string
	Values    []string
	Timestamp []int64
	Focused   int
}

func max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func draw_all(state *ProgramState) {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	width, height := termbox.Size()

	// Draw the buffer
	for i := state.offset; i < state.offset+height-1; i++ {
		if i > state.selected_buffer_line {
			break
		}
		for j, c := range state.buffer[i] {
			termbox.SetCell(j, i-state.offset, c, termbox.ColorDefault, termbox.ColorDefault)
		}
	}

	// Draw the divider line
	for i := 0; i < height; i++ {
		termbox.SetCell(width/2, i, '|', termbox.ColorDefault, termbox.ColorDefault)
	}

	// Draw the variables
	i := 0
	for _, v := range state.log {
		if !state.realtime && state.timecursor < v.Timestamp[0] {
			continue
		}

		for j, c := range v.Name {
			termbox.SetCell(width/2+2+j, i, c, termbox.ColorDefault, termbox.ColorDefault)
		}
		for j, c := range v.Values[v.Focused] {
			termbox.SetCell(width/2+2+j+len(v.Name)+1, i, c, termbox.ColorDefault, termbox.ColorDefault)
		}
		pos := fmt.Sprintf("%d/%d", v.Focused+1, len(v.Values))
		for j, c := range pos {
			termbox.SetCell(width-len(pos)+j, i, c, termbox.ColorDefault, termbox.ColorDefault)
		}

		i += 1
	}

	// Draw the selected index
	termbox.SetCell(width/2+1, state.selected_index, '>', termbox.ColorDefault, termbox.ColorDefault)

	status := "Running"
	extra := ""
	if !state.realtime {
		status = fmt.Sprintf("Frozen @ %0.2fs", float64(state.timecursor-state.start_time)/float64(1e9))
		extra = ", [ctrl-r] resume"
	}

	var full_status string
	if state.focused_pane == 0 {
		full_status = fmt.Sprintf("%s. [space] switch pane, [up/down] scroll%s", status, extra)
	} else {
		full_status = fmt.Sprintf("%s. [space] toggle pane, [up/down] select variable, [left/right] select frame%s", status, extra)
	}

	// Draw the selection box indicating which pane we're focused on
	for i := 0; i < width/2; i++ {
		c := ' '
		if i < len(full_status) {
			c = rune(full_status[i])
		}
		termbox.SetCell(state.focused_pane*((width/2)+1)+i, height-1, c, termbox.ColorBlack, termbox.ColorWhite)
	}

	termbox.Flush()
}

func seek_to_time(timestamp int64, variables *map[string]*HistoricalVariable) {
	for _, v := range *variables {
		// If the timestamp is before this current variable
		if v.Focused > 0 && v.Timestamp[v.Focused] > timestamp {
			for {
				v.Focused = max(0, v.Focused-1)
				if v.Focused == 0 || timestamp >= v.Timestamp[v.Focused] {
					break
				}
			}
		} else
		// If the timestamp is later than this current variable
		// increase the focus until the next one is after
		if v.Focused < len(v.Values)-1 && v.Timestamp[v.Focused+1] <= timestamp {
			for {
				v.Focused = min(v.Focused+1, len(v.Values)-1)
				if v.Focused == len(v.Values)-1 || v.Timestamp[v.Focused+1] >= timestamp {
					break
				}
			}
		}
	}
}

// This assumes that every timestamp exists in timestamps, otherwise
// this logic is too stupid at the moment and will crash beyond the ends of the
// array bounds
func scroll_to_time(timestamp int64, current_offset int, timestamps *[]int64) int {
	for timestamp > (*timestamps)[current_offset] {
		current_offset += 1
	}
	for timestamp < (*timestamps)[current_offset] {
		current_offset -= 1
	}
	return current_offset
}

func main() {
	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()
	termbox.SetInputMode(termbox.InputEsc | termbox.InputMouse)

	state := ProgramState{
		0,
		[]string{},
		[]int64{},
		map[string]*HistoricalVariable{},
		1,
		0,
		0,
		int64(0),
		true,
		time.Now().UnixNano(),
	}

	go func() {
		bio := bufio.NewReader(os.Stdin)
		for {
			line, _, err := bio.ReadLine()
			if err != nil {
				break
			}
			timestamp := time.Now().UnixNano()
			if len(line) > 0 && line[0] == '.' {
				slices := strings.SplitN(strings.TrimPrefix(string(line), "."), " ", 2)
				if len(slices) == 2 {
					existing := state.log[slices[0]]
					if existing == nil {
						existing = &HistoricalVariable{slices[0],
							[]string{},
							[]int64{},
							0}
						state.log[slices[0]] = existing
					}
					existing.Values = append(existing.Values, slices[1])
					existing.Timestamp = append(existing.Timestamp, timestamp)
					// Update the existing thing to realtime
					if state.realtime {
						existing.Focused = len(existing.Values) - 1
					}
				}
			}
			state.buffer = append(state.buffer, string(line))
			state.timestamps = append(state.timestamps, timestamp)
			if state.realtime {
				_, height := termbox.Size()
				state.offset = max(len(state.buffer)-height+1, 0)
				state.selected_buffer_line = len(state.buffer) - 1
			}
			draw_all(&state)
		}
	}()

loop:
	for {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			switch ev.Key {
			case termbox.KeyEsc:
				break loop
			case termbox.KeyArrowDown:
				switch state.focused_pane {
				case 0:
					state.realtime = false
					state.selected_buffer_line = min(state.selected_buffer_line+1, len(state.buffer)-1)

					_, height := termbox.Size()
					state.timecursor = state.timestamps[state.selected_buffer_line]
					state.offset = max(state.selected_buffer_line-height+2, 0)
					seek_to_time(state.timecursor, &state.log)
				case 1:
					state.selected_index = min(state.selected_index+1, len(state.log)-1)
				}
			case termbox.KeyArrowUp:
				switch state.focused_pane {
				case 0:
					state.realtime = false
					state.selected_buffer_line = max(0, state.selected_buffer_line-1)

					_, height := termbox.Size()
					state.timecursor = state.timestamps[state.selected_buffer_line]
					state.offset = max(state.selected_buffer_line-height+2, 0)
					seek_to_time(state.timecursor, &state.log)
				case 1:
					state.selected_index = max(0, state.selected_index-1)
				}
			case termbox.KeyArrowLeft:
				if state.focused_pane == 1 {
					state.realtime = false
					i := 0

					// Figure out where the new timecursor should be
					for _, v := range state.log {
						if i == state.selected_index {
							v.Focused = max(0, min(len(v.Values)-1, v.Focused-1))
							state.timecursor = v.Timestamp[v.Focused]
							break
						}
						i += 1
					}

					// Update
					_, height := termbox.Size()
					state.selected_buffer_line = scroll_to_time(state.timecursor, state.selected_buffer_line, &state.timestamps)
					state.offset = max(state.selected_buffer_line-height+1, 0)
					seek_to_time(state.timecursor, &state.log)
				}
			case termbox.KeyArrowRight:
				if state.focused_pane == 1 {
					state.realtime = false
					i := 0

					// Figure  out where the new timecursor should be
					for _, v := range state.log {
						if i == state.selected_index {
							v.Focused = max(0, min(len(v.Values)-1, v.Focused+1))
							state.timecursor = v.Timestamp[v.Focused]
							break
						}
						i += 1
					}

					// Update
					_, height := termbox.Size()
					state.selected_buffer_line = scroll_to_time(state.timecursor, state.selected_buffer_line, &state.timestamps)
					state.offset = max(state.selected_buffer_line-height+1, 0)
					seek_to_time(state.timecursor, &state.log)
				}
			case termbox.KeySpace:
				state.focused_pane = (state.focused_pane + 1) % 2
			case termbox.KeyCtrlR:
				state.realtime = true
				for _, v := range state.log {
					v.Focused = len(v.Values) - 1
				}
				state.selected_buffer_line = len(state.buffer) - 1
			}
			draw_all(&state)
		case termbox.EventMouse:
		case termbox.EventResize:
			draw_all(&state)
		}
	}
}
