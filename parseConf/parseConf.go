/*
 * ----------------------------------------------------------------------------
    Copyright (C) 2020  Evuraan, <evuraan@gmail.com>

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>.
 * ----------------------------------------------------------------------------
*/

package parseConf

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	conf      = "/etc/ipv6thingy.conf"
	leaseFile = "/var/lib/dhcp/dhclient6.leases"
)

type Confstruct struct {
	Init        bool
	Internal    string
	External    string
	V6Clients   map[string]bool
	Conf        string
	TemplatePtr *[]string
	Prefix      string
	RunThese    []string
}

var (
	Conf = Confstruct{Conf: conf}
)

func (ConfstructPtr *Confstruct) DoInit() bool {
	self := ConfstructPtr
	if self.Init {
		return self.Init
	}

	var lines []string
	var allLines []string

	func() {
		f, err := os.Open(self.Conf)
		if err != nil {
			fmt.Println("Err 33.1 - unable to open", self.Conf)
			panic(err)
		}
		defer f.Close()
		fscanner := bufio.NewScanner(f)
		for fscanner.Scan() {
			line := fscanner.Text()
			if len(line) < 1 {
				continue
			}
			allLines = append(allLines, line)

			if line[0:1] != "#" {
				lines = append(lines, line)
			}
		}
	}()

	if len(lines) < 1 {
		fmt.Println("Err 41.1 - no conf lines found..")
		panic("no conf lines found..")
	}

	tempDict := make(map[string]bool)

	devThings := getNetDev()
	for _, i := range lines {
		if strings.HasPrefix(i, "internal") {
			splat := strings.Split(i, "=")
			if len(splat) > 1 {
				trimmed := strings.TrimSpace(splat[1])
				_, ok := devThings[trimmed]
				if !ok {
					fmt.Printf("Err 13.1 - Could not find %v in proc devices\n", trimmed)
					panic(trimmed)
				}
				self.Internal = trimmed
				continue
			}
		}

		if strings.HasPrefix(i, "external") {
			splat := strings.Split(i, "=")
			if len(splat) > 1 {
				trimmed := strings.TrimSpace(splat[1])
				_, ok := devThings[trimmed]
				if !ok {
					fmt.Printf("Err 13.1 - Could not find %v in proc devices\n", trimmed)
					panic(trimmed)
				}
				self.External = trimmed
				continue
			}
		}

		if strings.HasPrefix(i, "enable ") {
			i = strings.ToLower(i)
			splat := strings.FieldsFunc(i, split)
			for _, y := range splat {
				if strings.HasPrefix(y, "fe80:") {
					x := strings.TrimSpace(y)
					tempDict[x] = true
				}
			}
		}
	}

	if len(tempDict) > 0 {
		self.V6Clients = tempDict
	}

	radvdTemplate := []string{}
	pegStart := false
	for x := range allLines {
		y := allLines[x] + "\n"
		if strings.HasPrefix(y, "#radvdStart") {
			pegStart = true
		}
		if pegStart {
			radvdTemplate = append(radvdTemplate, y)
		}
		if strings.HasPrefix(y, "#radvdEnd") {
			pegStart = false
		}
	}

	// we must have these -
	if len(self.Internal) > 0 && len(self.External) > 0 && len(radvdTemplate) > 0 {
		self.TemplatePtr = &radvdTemplate
		self.Init = true
	}

	return self.Init
}

func split(r rune) bool {
	return r == ' ' || r == '\t'
}

func (ConfstructPtr *Confstruct) GetConf() *Confstruct {
	self := ConfstructPtr
	if self.Init {
		return self
	} else if self.DoInit() {
		return self
	}
	return nil
}

// expand our Confstruct, and write to a conf file.
func (ConfstructPtr *Confstruct) WriteConf(fileName string) bool {
	self := ConfstructPtr
	template := *self.TemplatePtr
	templateText := strings.Join(template, "")
	templateText = strings.ReplaceAll(templateText, "INTERNALINTERFACE", self.Internal)
	templateText = strings.ReplaceAll(templateText, "#radvdEnd", "")
	templateText = strings.ReplaceAll(templateText, "#radvdStart", "")
	templateText = strings.ReplaceAll(templateText, "MYPREFIX", self.Prefix)

	if len(self.V6Clients) > 0 {
		clientThingy := "clients {"
		for k, _ := range self.V6Clients {
			clientThingy = fmt.Sprintf("%s\n\t\t%s;", clientThingy, k)
		}
		clientThingy += "\n	};"
		templateText = strings.ReplaceAll(templateText, "#CLIENTS_IF_ANY", clientThingy)
	} else {
		templateText = strings.ReplaceAll(templateText, "#CLIENTS_IF_ANY", "")
	}

	now := time.Now()
	writeThis := fmt.Sprintf("# Generated from %s on %v\n# Any local changes will be overwritten\n", conf, now.Format("2006-01-02 15:04:05"))
	manPage := "# See man radvd.conf, or https://linux.die.net/man/5/radvd.conf\n"
	writeThis = fmt.Sprintf("%s%s%s", writeThis, manPage, templateText)

	f, err := os.Create(fileName)
	if err != nil {
		fmt.Println("Err creating", fileName, err)
		return false
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	x, err := w.WriteString(writeThis)
	if err != nil {
		fmt.Println("Failed to write", fileName, err)
		return false
	}
	w.Flush()
	//f.Sync()

	fmt.Printf("Wrote %d bytes to %s\n", x, fileName)

	return true
}

//func getPrefix() (string, error) {
func (ConfstructPtr *Confstruct) GetPrefix() bool {
	self := ConfstructPtr
	f, err := os.Open(leaseFile)
	if err != nil {
		fmt.Println("err open leaseFile", err)
		return false
	}
	defer f.Close()
	fscanner := bufio.NewScanner(f)
	tempMap := make(map[int]string)
	i := 0
	for fscanner.Scan() {
		line := fscanner.Text()
		if strings.Contains(line, "iaprefix") {
			splat := strings.Split(line, " ")
			for _, x := range splat {
				if strings.Contains(x, "::") {
					//tempMap = append(tempMap, x)
					tempMap[i] = x
					i++
					break
				}
			}
		}
	}

	/*
	   cmd1 = "sysctl -w net.ipv6.conf.{externalInterface}.accept_ra=2".format(externalInterface=externalInterface)
	   cmd2 = "sysctl -w net.ipv6.conf.{externalInterface}.forwarding=0".format(externalInterface=externalInterface)
	   cmd3 = "ip -6 route add {PREFIX} dev {internalInterface}".format(internalInterface=internalInterface, PREFIX=PREFIX)
	*/

	x := len(tempMap)
	if x > 0 {
		lease := tempMap[x-1]
		self.Prefix = lease
		self.RunThese = []string{}
		cmd3 := fmt.Sprintf("ip -6 route add %s dev %s", self.Prefix, self.Internal)
		self.RunThese = append(self.RunThese, cmd3)
		return true
	} else {
		fmt.Println("iaprefix null")
		return false
	}
}

func main() {
	fmt.Println("Hello!")
	//Conf.DoInit()
	Conf.GetConf()
}
