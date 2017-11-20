package main

import (
	"bufio"
	"fmt"
	"github.com/fhs/gompd/mpd"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var conn *mpd.Client
var browserWinid int
var currentPath string
var quit bool

type event struct {
	middlemouse bool
	text        string
}

func createMpdConn() *mpd.Client {
	mpdHost, hostEnvSet := os.LookupEnv("MPD_HOST")
	mpdPort, portEnvSet := os.LookupEnv("MPD_PORT")
	mpdPassword := ""

	// extract password of MPD_HOST
	if hostEnvSet {
		if strings.Contains(mpdHost, "@") {
			mpdPassword = strings.Split(mpdHost, "@")[0]
			mpdHost = strings.Split(mpdHost, "@")[1]
		}
	} else {
		mpdHost = "localhost"
	}

	// default to mpd's default port
	if !portEnvSet {
		mpdPort = "6600"
	}

	var err error

	if mpdPassword == "" {
		conn, err = mpd.Dial("tcp", mpdHost+":"+mpdPort)
	} else {
		conn, err = mpd.DialAuthenticated("tcp", mpdHost+":"+mpdPort, mpdPassword)
	}
	if err != nil {
		panic(err)
	}
	return conn
}

func bodyLength(winid int) int {
	dat, err := ioutil.ReadFile(fmt.Sprintf("/mnt/acme/%d/ctl", winid))
	if err != nil {
		panic(err)
	}

	ctlString := strings.TrimSpace(string(dat))

	i, err := strconv.Atoi(strings.Fields(ctlString)[2])
	if err != nil {
		panic(err)
	}

	return i
}

func clearBody(winid int, file *os.File) {
	len := bodyLength(winid)
	file.WriteString(strings.Repeat("\b", len))
}

func createWindow() int {
	dat, err := ioutil.ReadFile("/mnt/acme/new/ctl")
	if err != nil {
		panic(err)
	}

	ctlString := strings.TrimSpace(string(dat))

	i, err := strconv.Atoi(strings.Fields(ctlString)[0])
	if err != nil {
		panic(err)
	}

	return i
}

func deleteWindow(winid int) {
	file, err := os.OpenFile(fmt.Sprintf("/mnt/acme/%d/ctl", winid), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	file.WriteString("delete\n")
	file.Close()
}

func parseEvent(eventStr string) (event, bool) {
	parsed := false
	evt := event{false, "foo"}

	if eventStr != "" && eventStr[0] == 'M' {
		if eventStr[1] == 'x' || eventStr[1] == 'X' {
			evt.middlemouse = true
			out := strings.SplitAfterN(eventStr, " ", 5)
			evt.text = strings.Trim(out[4], " ")
			parsed = evt.text != ""
		} else if eventStr[1] == 'l' || eventStr[1] == 'L' {
			evt.middlemouse = false
			out := strings.SplitAfterN(eventStr, " ", 5)
			evt.text = strings.Trim(out[4], " ")
			parsed = evt.text != ""
		}
	}

	return evt, parsed
}

func mpdClosedConn(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "EOF") ||
		strings.Contains(err.Error(), "Hangup") ||
		strings.Contains(err.Error(), "broken pipe")

}

func writeName(winid int, name string) {
	file, err := os.OpenFile(fmt.Sprintf("/mnt/acme/%d/ctl", winid), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	file.WriteString(fmt.Sprintf("name %s\n", name))
	file.Close()
}

func writeTags(winid int, tags string) {
	file, err := os.OpenFile(fmt.Sprintf("/mnt/acme/%d/tag", winid), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	file.WriteString(tags)
	file.Close()
}

func absPathFromRelPath(wd string, relPath string) string {
	if relPath == "" {
		return wd
	}

	if relPath[0] == '/' {
		return relPath
	} else {
		return strings.TrimRight(wd, " /") + "/" + strings.Trim(relPath, "/ ")
	}
}

func relPathFromAbsPath(absPath string) string {
	path := strings.Trim(absPath, "/")
	slices := strings.Split(path, "/")
	return slices[len(slices)-1]
}

func convertForPrint(path string) string {
	s := path
	s = strings.Replace(s, "(", "〔", -1)
	s = strings.Replace(s, ")", "〕", -1)
	s = strings.Replace(s, "&", "⊕", -1)
	s = strings.Replace(s, "?", "¿", -1)
	s = strings.Replace(s, "'", "´", -1)
	s = strings.Replace(s, "[", "【", -1)
	s = strings.Replace(s, "]", "】", -1)
	s = strings.Replace(s, ":", "᛬", -1)
	s = strings.Replace(s, "<", "〈", -1)
	s = strings.Replace(s, ">", "〉", -1)
	s = strings.Replace(s, "+", "±", -1)
	s = strings.Replace(s, ".", "。", -1)
	s = strings.Replace(s, "-", "‒", -1)
	s = strings.Replace(s, ",", "、", -1)
	s = strings.Replace(s, " ", "⋯", -1)
	return s
}

func convertFromPrint(s string) string {
	path := s
	path = strings.Replace(path, "〔", "(", -1)
	path = strings.Replace(path, "〕", ")", -1)
	path = strings.Replace(path, "⊕", "&", -1)
	path = strings.Replace(path, "¿", "?", -1)
	path = strings.Replace(path, "´", "'", -1)
	path = strings.Replace(path, "【", "[", -1)
	path = strings.Replace(path, "】", "]", -1)
	path = strings.Replace(path, "᛬", ":", -1)
	path = strings.Replace(path, "〈", "<", -1)
	path = strings.Replace(path, "〉", ">", -1)
	path = strings.Replace(path, "±", "+", -1)
	path = strings.Replace(path, "。", ".", -1)
	path = strings.Replace(path, "‒", "-", -1)
	path = strings.Replace(path, "、", ",", -1)
	path = strings.Replace(path, "⋯", " ", -1)
	return path
}

func showBrowser(bodyFile *os.File, uri string, winid int) bool {
	trimmedPath := strings.Trim(uri, "/")
	attrs, err := conn.ListInfo(trimmedPath)
	if err != nil {
		if mpdClosedConn(err) {
			conn = createMpdConn()
			attrs, err = conn.ListInfo(trimmedPath)
			if err != nil {
				return false
			}
		} else {
			return false
		}
	}

	clearBody(winid, bodyFile)
	bodyFile.WriteString(fmt.Sprintf("current path: /%s\n", convertForPrint(trimmedPath)))
	for _, attr := range attrs {
		dir := attr["directory"]
		file := attr["file"]
		if dir != "" {
			bodyFile.WriteString(fmt.Sprintf("%s\n", convertForPrint(relPathFromAbsPath(dir))))
		} else {
			bodyFile.WriteString(fmt.Sprintf("%s\n", convertForPrint(relPathFromAbsPath(file))))
		}
	}
	return true
}

func showPlaylist(file *os.File) {
	attrs, err := conn.PlaylistInfo(-1, -1)
	if err != nil {
		if mpdClosedConn(err) {
			conn = createMpdConn()
			attrs, err = conn.PlaylistInfo(-1, -1)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			fmt.Println(err.Error())
		}
	}

	total := 0
	for _, attr := range attrs {
		artist := attr["Artist"]
		title := attr["Title"]
		duration, _ := strconv.Atoi(attr["Time"])
		total += duration
		minutes := duration / 60
		seconds := duration % 60
		if artist == "" || title == "" {
			file.WriteString(fmt.Sprintf("# %s # %s # %02d:%02d\n", attr["Pos"], attr["file"], minutes, seconds))
		} else {
			file.WriteString(fmt.Sprintf("# %s # %s - %s # %02d:%02d\n", attr["Pos"], attr["Artist"], attr["Title"], minutes, seconds))
		}
	}
	file.WriteString(fmt.Sprintf("TOTAL: %d:%02d\n", total/60, total%60))
}

func showStatus(file *os.File, refresh bool) {
	if refresh {
		file.WriteString("\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b")
	}
	attrs, err := conn.Status()
	if err != nil {
		if mpdClosedConn(err) {
			conn = createMpdConn()
			attrs, err = conn.Status()
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	file.WriteString(fmt.Sprintf("State: %-5s Song: %-5s Time: %-20s\n", attrs["state"], attrs["song"], attrs["time"]))
}

func readEvents(winid int, bodyFile *os.File) {
	browserWinid = -1
	var browserBody *os.File
	file, err := os.Open(fmt.Sprintf("/mnt/acme/%d/event", winid))
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for !quit && scanner.Scan() {
		evt, parsed := parseEvent(scanner.Text())
		if parsed {
			if evt.middlemouse {
				switch evt.text {
				case "Quit":
					quit = true
					deleteWindow(winid)
					if browserWinid != -1 {
						deleteWindow(browserWinid)
					}
					os.Exit(0)
				case "Play":
					err := conn.Play(-1)
					if mpdClosedConn(err) {
						conn = createMpdConn()
						conn.Play(-1)
					}
					refresh(winid, bodyFile, false)
				case "Stop":
					err := conn.Stop()
					if mpdClosedConn(err) {
						conn = createMpdConn()
						conn.Stop()
					}
					refresh(winid, bodyFile, false)
				case "Pause":
					err := conn.Pause(true)
					if mpdClosedConn(err) {
						conn = createMpdConn()
						conn.Pause(true)
					}
					refresh(winid, bodyFile, false)
				case "Clear":
					err := conn.Clear()
					if mpdClosedConn(err) {
						conn = createMpdConn()
						conn.Clear()
					}
					refresh(winid, bodyFile, true)
				case "Shuffle":
					err := conn.Shuffle(-1, -1)
					if mpdClosedConn(err) {
						conn = createMpdConn()
						conn.Shuffle(-1, -1)
					}
					refresh(winid, bodyFile, true)
				case "Consume":
					err := conn.Consume(true)
					if mpdClosedConn(err) {
						conn = createMpdConn()
						conn.Consume(true)
					}
					refresh(winid, bodyFile, false)
				case "NoConsume":
					err := conn.Consume(false)
					if mpdClosedConn(err) {
						conn = createMpdConn()
						conn.Consume(false)
					}
					refresh(winid, bodyFile, false)
				case "Refresh":
					refresh(winid, bodyFile, true)
				case "Browse":
					if browserWinid == -1 {
						browserWinid, browserBody = createNewBrowser("/", winid, bodyFile)
					} else {
						currentPath = "/"
						showBrowser(browserBody, currentPath, browserWinid)
					}
				default:
					if strings.HasPrefix(evt.text, "Move") {
						slices := strings.Fields(evt.text)
						if len(slices) != 3 {
							break // malfromed Move <from> <to> command
						}
						i, err := strconv.Atoi(slices[1])
						if err == nil {
							u, err := strconv.Atoi(slices[2])
							if err == nil {
								err = conn.Move(i, -1, u)
								if mpdClosedConn(err) {
									conn = createMpdConn()
									conn.Move(i, -1, u)
								}
								refresh(winid, bodyFile, true)
							}
						}
					} else if strings.HasPrefix(evt.text, "Del") {
						slices := strings.Fields(evt.text)
						if len(slices) != 2 {
							break // malfromed Del <position>
						}
						i, err := strconv.Atoi(slices[1])
						if err == nil {
							err = conn.Delete(i, -1)
							if mpdClosedConn(err) {
								conn = createMpdConn()
								conn.Delete(i, -1)
							}
							refresh(winid, bodyFile, true)
						}
					} else if strings.HasPrefix(evt.text, "rDel") {
						slices := strings.Fields(evt.text)
						if len(slices) != 3 {
							break // malfromed rDel <from> <to> command
						}
						i, err := strconv.Atoi(slices[1])
						if err == nil {
							u, err := strconv.Atoi(slices[2])
							if err == nil {
								err = conn.Delete(i, u)
								if mpdClosedConn(err) {
									conn = createMpdConn()
									conn.Delete(i, u)
								}
							}
							refresh(winid, bodyFile, true)
						}
					} else {
						i, err := strconv.Atoi(evt.text)
						if err == nil {
							err = conn.Play(i)
							if mpdClosedConn(err) {
								conn = createMpdConn()
								conn.Play(i)
							}
							refresh(winid, bodyFile, true)
						}
					}
				}
			} else {
				i, err := strconv.Atoi(evt.text)
				if err == nil {
					attrs, err := conn.PlaylistInfo(i, -1)
					if err != nil {
						if mpdClosedConn(err) {
							conn = createMpdConn()
							attrs, err = conn.PlaylistInfo(i, -1)
							if err != nil {
								log.Fatal(err)
							}
						} else {
							fmt.Println(err.Error())
						}
					}

					slices := strings.Split(attrs[0]["file"], "/")
					filePath := ""
					for i := 0; i < (len(slices) - 1); i++ {
						filePath += "/" + slices[i]
					}
					if browserWinid == -1 {
						browserWinid, browserBody = createNewBrowser(filePath, winid, bodyFile)
					} else {
						currentPath = filePath
						showBrowser(browserBody, currentPath, browserWinid)
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

func createNewBrowser(filePath string, queueWinid int, queueBody *os.File) (int, *os.File) {
	winid := createWindow()

	file, err := os.OpenFile(fmt.Sprintf("/mnt/acme/%d/body", winid), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	writeName(winid, "browse:")
	writeTags(winid, "Close Update Info")
	if showBrowser(file, filePath, winid) {
		currentPath = filePath
	} else {
		return -1, nil
	}
	go readBrowserEvents(winid, file, queueWinid, queueBody)
	return winid, file
}

func readBrowserEvents(winid int, bodyFile *os.File, queueWinid int, queueBody *os.File) {
	file, err := os.Open(fmt.Sprintf("/mnt/acme/%d/event", winid))
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for !quit && scanner.Scan() {
		evt, parsed := parseEvent(scanner.Text())
		if parsed {
			evt.text = convertFromPrint(evt.text)
			if evt.middlemouse {
				switch evt.text {
				case "Close":
					deleteWindow(winid)
					browserWinid = -1
					return
				case "Update":
					path := strings.Trim(currentPath, " /")
					_, err := conn.Update(path)
					fmt.Println("Updating")
					if mpdClosedConn(err) {
						conn = createMpdConn()
						_, err = conn.Update(path)
					}

					if err != nil {
						fmt.Fprintf(os.Stderr, "Failure to update path '%s': %s\n", path, err.Error())
					}
				default:
					if strings.HasPrefix(evt.text, "Info") {
						slices := strings.Fields(evt.text)
						if len(slices) < 2 {
							return // malfromed Info <filePath>
						}
						relPath := ""
						for i := 1; i < len(slices); i++ {
							relPath += " " + slices[i]
						}
						filePath := strings.Trim(absPathFromRelPath(currentPath, relPath), " /")
						cmd := exec.Command("songinfo", filePath)
						newwinid := createWindow()
						file, err := os.OpenFile(fmt.Sprintf("/mnt/acme/%d/body", newwinid), os.O_APPEND|os.O_WRONLY, 0600)
						writeName(newwinid, "/tmp/songinfo")
						writeTags(newwinid, "Delete")
						if err != nil {
							panic(err)
						}
						cmd.Stdout = file
						err = cmd.Start()
						if err != nil {
							fmt.Println(err)
						}
						cmd.Wait()
						file.Close()
					} else {
						filePath := strings.Trim(absPathFromRelPath(currentPath, evt.text), " /")
						err := conn.Add(filePath)
						if mpdClosedConn(err) {
							conn = createMpdConn()
							err = conn.Add(filePath)
						}
						if err == nil {
							refresh(queueWinid, queueBody, true)
						}
					}
				}
			} else {
				newPath := absPathFromRelPath(currentPath, evt.text)
				// only change path on success
				if showBrowser(bodyFile, newPath, winid) {
					currentPath = newPath
				}
			}
		}
	}

	if err := scanner.Err(); err != nil && !quit {
		panic(err)
	}
}

func refresh(winid int, bodyFile *os.File, full bool) {
	if full {
		clearBody(winid, bodyFile)
		showPlaylist(bodyFile)
	}
	showStatus(bodyFile, !full)
}

func main() {
	quit = false
	conn = createMpdConn()
	defer conn.Close()

	winid := createWindow()

	file, err := os.OpenFile(fmt.Sprintf("/mnt/acme/%d/body", winid), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	writeName(winid, "samc:")
	writeTags(winid, "Quit Clear Refresh Play Pause Stop Browse")

	showPlaylist(file)
	showStatus(file, false)

	readEvents(winid, file)

	file.Close()
}
