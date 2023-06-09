package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/schollz/progressbar/v3"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// SCP copy file to remote and exec command
func SCP(conf *SshConfig, filename string, commands ...string) error {
	var remote *ssh.Client
	var err error
	if conf.ConfigAlias != "" {
		remote, err = jumpRecursion(conf.ConfigAlias)
	} else {
		var auth []ssh.AuthMethod
		if conf.Keyfile != "" {
			auth = append(auth, publicKeyFile(conf.Keyfile))
		}
		if conf.Password != "" {
			auth = append(auth, ssh.Password(conf.Password))
		}
		sshConfig := &ssh.ClientConfig{
			User:            conf.User,
			Auth:            auth,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
		remote, err = ssh.Dial("tcp", conf.Addr, sshConfig)
	}
	if err != nil {
		log.Errorf("Dial INTO remote server error: %s", err)
		return err
	}

	sess, err := remote.NewSession()
	if err != nil {
		return err
	}
	err = main(sess, filename)
	if err != nil {
		return err
	}
	sess, err = remote.NewSession()
	if err != nil {
		return err
	}
	for _, command := range commands {
		output, err := sess.CombinedOutput(command)
		if err != nil {
			fmt.Fprint(os.Stderr, string(output))
			return err
		} else {
			fmt.Fprint(os.Stdout, string(output))
		}
	}
	return nil
}

// https://blog.neilpang.com/%E6%94%B6%E8%97%8F-scp-secure-copy%E5%8D%8F%E8%AE%AE/
func main(sess *ssh.Session, filename string) error {
	open, err := os.Open(filename)
	if err != nil {
		return err
	}
	stat, err := open.Stat()
	if err != nil {
		return err
	}
	defer open.Close()
	defer sess.Close()
	go func() {
		w, _ := sess.StdinPipe()
		defer w.Close()
		fmt.Fprintln(w, "D0755", 0, "kubevpndir") // mkdir
		fmt.Fprintln(w, "C0644", stat.Size(), filepath.Base(filename))
		err := sCopy(w, open, stat.Size())
		if err != nil {
			log.Errorf("failed to transfer file to remote: %v", err)
			return
		}
		fmt.Fprint(w, "\x00") // transfer end with \x00
	}()
	return sess.Run("scp -tr ./")
}

func sCopy(dst io.Writer, src io.Reader, size int64) error {
	total := float64(size) / 1024 / 1024
	fmt.Printf("Length: 68276642 (%0.2fM)\n", total)

	bar := progressbar.NewOptions(int(size),
		progressbar.OptionSetWriter(os.Stdout),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(50),
		progressbar.OptionOnCompletion(func() {
			_, _ = fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionSetDescription("Transferring image file..."),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))
	buf := make([]byte, 10<<(10*2)) // 10M
	written, err := io.CopyBuffer(io.MultiWriter(dst, bar), src, buf)
	if err != nil {
		return err
	}
	if written != size {
		err = fmt.Errorf("failed to transfer file to remote: written size %d but actuall is %d", written, size)
		return err
	}
	return nil
}
