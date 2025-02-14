// Copyright 2019-2021 Graham Clark. All rights reserved.  Use of this source
// code is governed by the MIT license that can be found in the LICENSE
// file.

// +build !windows
// +build !darwin

package system

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"syscall"

	log "github.com/sirupsen/logrus"
)

//======================================================================

var fdre *regexp.Regexp = regexp.MustCompile(`/dev/fd/([[:digit:]]+)`)

// DumpcapExt will run dumpcap first, but if it fails, run tshark. Intended as
// a special case to allow termshark -i <iface> to use dumpcap if possible,
// but if it fails (e.g. iface==randpkt), fall back to tshark. dumpcap is more
// efficient than tshark at just capturing, and will drop fewer packets, but
// tshark supports extcap interfaces.
func DumpcapExt(dumpcapBin string, tsharkBin string, args ...string) error {
	var err error

	// If the first argument is /dev/fd/X, it means the process should have
	// descriptor X open and will expect packet data to be readable on it.
	// This /dev/fd feature does not work on tshark when run on freebsd, meaning
	// tshark will fail if you do something like
	//
	// cat foo.pcap | tshark -r /dev/fd/0
	//
	// The fix here is to replace /dev/fd/X with the arg "-", which tshark will
	// interpret as stdin, then dup descriptor X to 0 before starting dumpcap/tshark
	//
	if len(args) >= 2 {
		if os.Getenv("TERMSHARK_REPLACE_DEVFD") != "0" {
			fdnum := fdre.FindStringSubmatch(args[1])
			if len(fdnum) == 2 {
				fd, err := strconv.Atoi(fdnum[1])
				if err != nil {
					log.Warnf("Unexpected error parsing %s: %v", args[1], err)
				} else {
					err = Dup2(fd, 0)
					if err != nil {
						log.Warnf("Problem duplicating fd %d to 0: %v", fd, err)
						log.Warnf("Will not try to replace argument %s to tshark", args[1])
					} else {
						log.Infof("Replacing argument %s with - for tshark compatibility", args[1])
						args[1] = "-"
					}
				}
			}
		}
	}

	dumpcapCmd := exec.Command(dumpcapBin, args...)
	log.Infof("Starting dumpcap command %v", dumpcapCmd)
	dumpcapCmd.Stdin = os.Stdin
	dumpcapCmd.Stdout = os.Stdout
	dumpcapCmd.Stderr = os.Stderr
	if dumpcapCmd.Run() != nil {
		var tshark string
		tshark, err = exec.LookPath(tsharkBin)
		if err == nil {
			log.Infof("Retrying with dumpcap command %v", append([]string{tshark}, args...))
			err = syscall.Exec(tshark, append([]string{tshark}, args...), os.Environ())
		}
	}

	return err
}
