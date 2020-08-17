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

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
	"v6/parseConf"
)

var (
	conf         *parseConf.Confstruct
	cmdList      = []string{"radvd", "systemctl", "ip", "sysctl", "dhclient"}
	validReasons = map[string]bool{"BOUND6": true, "REBIND6": true}
	mode         = 0
	reason       = ""
	iface        = ""
)

const (
	name      = "ipv6Thingy"
	ver       = "1.4h"
	verInfo   = name + " " + ver
	leaseFile = "/var/lib/dhcp/dhclient6.leases"
	radvdConf = "/etc/radvd.conf"
)

func main() {

	// usr/local/bin/ipv6.py $reason $interface  || : -- helper
	//

	fmt.Printf("%s Copyright (C) 2020 Evuraan <evuraan@gmail.com>\n", verInfo)
	fmt.Println("This program comes with ABSOLUTELY NO WARRANTY.\n")
	argc := len(os.Args)
	if argc == 3 {
		reason = os.Args[1]
		iface = os.Args[2]
		mode = 1
	} else if argc == 2 {
		if os.Args[1] == "--daemon" {
			mode = 2
		} else {
			fmt.Println("Invalid operation..")
			os.Exit(1)
		}
	} else {
		fmt.Println("Invalid operation..")
		os.Exit(1)
	}

	// the commands that we need for later - are they avaialable?
	for _, cmd := range cmdList {
		if !checkExec(cmd) {
			fmt.Println("Error - 31.1 - cannot find", cmd)
			panic(cmd)
		}
	}

	conf = parseConf.Conf.GetConf()
	//fmt.Printf("conf type: %T\n", conf)
	if conf == nil {
		panic("Err 33.3 - conf is empty")
	}

	switch mode {
	case 1:
		fmt.Printf("Helper - reason: %s, Iface: %s\n", reason, iface)
		if iface != conf.External {
			fmt.Printf("Ignoring, NIC %s not external\n", iface)
			os.Exit(0)
		}
		checkReason := strings.ToUpper(reason)
		_, ok := validReasons[checkReason]
		if !ok {
			fmt.Printf("Ignoring - reason %s not valid\n", checkReason)
			os.Exit(0)
		}
		helper()
	case 2:
		fmt.Println("Daemon!")
		daemon()
	default:
		fmt.Println("Invalid operation")
	}

}

func helper() {

	if !conf.GetPrefix() {
		fmt.Println("Failed on prefix")
		panic("Failed on prefix - 33.551")
	}

	tag := time.Now().UnixNano()
	tempFile := fmt.Sprintf("/tmp/temp-%v", tag)
	if !conf.WriteConf(tempFile) {
		sayThis := fmt.Sprintf("Failed to write %s", tempFile)
		err := errors.New(sayThis)
		panic(err)
	}

	verifyCmd := fmt.Sprintf("radvd -c -C %s", tempFile)
	cmderr := runThis(verifyCmd)
	if cmderr != nil {
		fmt.Printf("Failed verification of %s\n", tempFile)
		panic(cmderr)
	}

	backupFile := fmt.Sprintf("/etc/backup-radvd-%v", tag)
	err := copyFile(radvdConf, backupFile)
	if err != nil {
		fmt.Printf("Failed to backup %s to %s\n", radvdConf, backupFile)
	} else {
		fmt.Printf("Backup: %s saved as %s\n", radvdConf, backupFile)
	}

	err = copyFile(tempFile, radvdConf)
	if err != nil {
		fmt.Printf("Failed copying from %s to %s\n", tempFile, radvdConf)
		panic(err)
	}

	for i := range conf.RunThese {
		reqCmd := conf.RunThese[i]
		fmt.Println("running:", reqCmd)
		_ = runThis(reqCmd)
		// revisit this sometime
	}

	//restartCmd := "/bin/echo systemctl restart radvd"
	restartCmd := "systemctl restart radvd"
	restartErr := runThis(restartCmd)
	if restartErr != nil {
		fmt.Printf("Failed to restart radvd. Please intervene.\n")
		panic(restartErr)
	}

	fmt.Println("radvd restarted OK")
	go os.Remove(tempFile)
}

func daemon() {
	// We need root access.
	if os.Geteuid() != 0 {
		fmt.Println("We need root access for daemon mode..")
		panic("need root access")
	}

	dhclientCMD := "dhclient -6 -P -d -v " + conf.External

	//dhclientCMD := "/tmp/x.sh"
	/*
			          cmd1 = "sysctl -w net.ipv6.conf.{externalInterface}.accept_ra=2".format(externalInterface=externalInterface)
		         	  cmd2 = "sysctl -w net.ipv6.conf.{externalInterface}.forwarding=0".format(externalInterface=externalInterface)
	*/

	for {
		// https://lists.debian.org/debian-ipv6/2011/05/msg00046.html
		cmd1 := fmt.Sprintf("sysctl -w net.ipv6.conf.%s.accept_ra=2", conf.External)
		cmd2 := fmt.Sprintf("sysctl -w net.ipv6.conf.%s.forwarding=0", conf.External)
		err := runThis(cmd1)
		if err != nil {
			fmt.Println("Failed to run", cmd1)
			panic(err)
		}
		err = runThis(cmd2)
		if err != nil {
			fmt.Println("Failed to run", cmd2)
			panic(err)
		}

		runThis(dhclientCMD)
		time.Sleep(3 * time.Second)
	}
}

func runThis(cmdIn string) error {
	cmdSplat := strings.Split(cmdIn, " ")
	cmd := exec.Command(cmdSplat[0], cmdSplat[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println("runThis", cmdIn, err)
	}

	return err
}

func checkExec(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func getPrefix() (string, error) {
	f, err := os.Open(leaseFile)
	if err != nil {
		return "", err
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

	x := len(tempMap)
	if x > 0 {
		return tempMap[x-1], nil
	} else {
		err1 := errors.New("iaprefix null")
		return "", err1
	}
}

func copyFile(srcFile, dstFile string) error {

	src, err := os.Open(srcFile)
	if err != nil {
		return (err)
	}
	defer src.Close()
	dst, err := os.Create(dstFile)
	if err != nil {
		return (err)
	}
	defer dst.Close()
	n, err := io.Copy(dst, src)
	if err != nil {
		return (err)
	}
	fmt.Printf("Copied %d bytes from %s to %s\n", n, srcFile, dstFile)
	return nil
}
