
package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/smtp"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/disk"
)

var (
	host       = "smtp.gmail.com"
	username   = ""
	password   = ""
	portNumber = "587"
)

type Memory struct {
	MemTotal     float64 `json:"total"`
	MemFree      float64 `json:"free"`
	MemAvailable float64 `json:"avilable"`
	MemPercent   int     `json:"mempercent"`
}

type Sender struct {
	auth smtp.Auth
}

type Message struct {
	To          []string
	CC          []string
	BCC         []string
	Subject     string
	Body        string
	Attachments map[string][]byte
}

func New() *Sender {
	auth := smtp.PlainAuth("", username, password, host)
	return &Sender{auth}
}

func (s *Sender) Send(m *Message) error {
	return smtp.SendMail(fmt.Sprintf("%s:%s", host, portNumber), s.auth, username, m.To, m.ToBytes())
}

func NewMessage(s, b string) *Message {
	return &Message{Subject: s, Body: b, Attachments: make(map[string][]byte)}
}

func (m *Message) AttachFile(src string) error {
	b, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	_, fileName := filepath.Split(src)
	m.Attachments[fileName] = b
	return nil
}

func (m *Message) ToBytes() []byte {
	buf := bytes.NewBuffer(nil)
	withAttachments := len(m.Attachments) > 0
	buf.WriteString(fmt.Sprintf("Subject: %s\n", m.Subject))
	buf.WriteString(fmt.Sprintf("To: %s\n", strings.Join(m.To, ",")))
	if len(m.CC) > 0 {
		buf.WriteString(fmt.Sprintf("Cc: %s\n", strings.Join(m.CC, ",")))
	}

	if len(m.BCC) > 0 {
		buf.WriteString(fmt.Sprintf("Bcc: %s\n", strings.Join(m.BCC, ",")))
	}

	buf.WriteString("MIME-Version: 1.0\n")
	writer := multipart.NewWriter(buf)
	boundary := writer.Boundary()
	if withAttachments {
		buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\n", boundary))
		buf.WriteString(fmt.Sprintf("--%s\n", boundary))
	} else {
		buf.WriteString("Content-Type: text/plain; charset=utf-8\n")
	}

	buf.WriteString(m.Body)
	if withAttachments {
		for k, v := range m.Attachments {
			buf.WriteString(fmt.Sprintf("\n\n--%s\n", boundary))
			buf.WriteString(fmt.Sprintf("Content-Type: %s\n", http.DetectContentType(v)))
			buf.WriteString("Content-Transfer-Encoding: base64\n")
			buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=%s\n", k))

			b := make([]byte, base64.StdEncoding.EncodedLen(len(v)))
			base64.StdEncoding.Encode(b, v)
			buf.Write(b)
			buf.WriteString(fmt.Sprintf("\n--%s", boundary))
		}

		buf.WriteString("--")
	}

	return buf.Bytes()
}
func GetDiskServices(path string) disk.UsageStat {
	diskInfo, _ := disk.Usage(path)
	return *diskInfo
}
func toInt(raw string) int {
	if raw == "" {
		return 0
	}
	res, err := strconv.Atoi(raw)
	if err != nil {
		panic(err)
	}
	return res
}
func parseLine(raw string) (key string, value int) {
	// fmt.Println(raw)
	text := strings.ReplaceAll(raw[:len(raw)-2], " ", "")
	keyValue := strings.Split(text, ":")
	return keyValue[0], toInt(keyValue[1])
}
func ReadMemoryStats() Memory {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	bufio.NewScanner(file)
	scanner := bufio.NewScanner(file)
	res := Memory{}
	for scanner.Scan() {
		key, value := parseLine(scanner.Text())
		switch key {
		case "MemTotal":
			res.MemTotal = float64(value)
		case "MemFree":
			res.MemFree = float64(value)
		case "MemAvailable":
			res.MemAvailable = float64(value)
		}
	}
	return res
}

const ShellToUse = "bash"

func Shellout(command string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command(ShellToUse, "-c", command)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
func main() {

	res := GetDiskServices("/")
	res.Free = res.Free / 1000000000
	fmt.Println(res.Free)

	memModel := ReadMemoryStats()
	memModel.MemAvailable = memModel.MemAvailable / 1000000
	memModel.MemFree = memModel.MemFree / 1000000
	memModel.MemTotal = memModel.MemTotal / 1000000
	memModel.MemPercent = int((100 - (memModel.MemAvailable/memModel.MemTotal)*100))

	b := []byte("free disk space: " + fmt.Sprint(res.Free) + " GB\n" + "Memory Percent: " + fmt.Sprint(memModel.MemPercent) + "% \n")
	err := ioutil.WriteFile("info.txt", b, 0644)
	if err != nil {
		log.Fatal(err)
	}
	Shellout("echo \" \" >> info.txt")
	Shellout("echo ------------- >> info.txt")

	Shellout("docker service ls >> info.txt")
	Shellout("echo \" \" >> info.txt")
	Shellout("echo ------------- >> info.txt")

	Shellout("docker node ls >> info.txt")
	Shellout("echo \" \" >> info.txt")
	Shellout("echo -------------PING >> info.txt")

	Shellout("ping -c 2 192.168.14.42 >> info.txt")
	Shellout("echo \" \" >> info.txt")
	Shellout("echo -------------PING >> info.txt")

	Shellout("ping -c 2 192.168.14.7 >> info.txt")

	sender := New()
	m := NewMessage("Test", "Body message.")
	m.To = []string{"", ""}
	m.CC = []string{""}
	m.BCC = []string{""}
	m.AttachFile("info.txt")
	fmt.Println(sender.Send(m))

}
