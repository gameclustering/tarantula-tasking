package util

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	scp "github.com/bramvdbogaerde/go-scp"
	"github.com/skeema/knownhosts"
	"golang.org/x/crypto/ssh"
)

type SshClient struct {
	Host       string
	User       string
	Password   string
	PrivateKey string
	KHFile     string

	conn *ssh.Client
}

func (c *SshClient) WithPassword() error {
	conf := ssh.ClientConfig{
		User: c.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(c.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	ci, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, 22), &conf)
	if err != nil {
		return err
	}
	c.conn = ci
	return nil
}

func (c *SshClient) WithKey() error {

	signer, err := ssh.ParsePrivateKey([]byte(c.PrivateKey))
	if err != nil {
		return err
	}

	hc, err := knownhosts.NewDB(c.KHFile)
	if err != nil {
		return err
	}
	cb := ssh.HostKeyCallback(func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		icc := hc.HostKeyCallback()
		err := icc(hostname, remote, key)
		if knownhosts.IsHostKeyChanged(err) {
			// VM recreated at same IP — replace stale entry with new host key
			if err2 := replaceKnownHost(c.KHFile, hostname, remote, key); err2 == nil {
				return nil
			}
			return err
		}
		if knownhosts.IsHostUnknown(err) {
			f, err := os.OpenFile(c.KHFile, os.O_APPEND|os.O_WRONLY, 0600)
			if err == nil {
				defer f.Close()
				if err = knownhosts.WriteKnownHost(f, hostname, remote, key); err != nil {
					return err
				}
			}
		}
		return nil
	})
	conf := ssh.ClientConfig{
		User: c.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: cb,
	}
	ci, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", c.Host, 22), &conf)
	if err != nil {
		return err
	}
	c.conn = ci
	return nil
}

func replaceKnownHost(file, hostname string, remote net.Addr, key ssh.PublicKey) error {
	host, _, _ := net.SplitHostPort(hostname)
	if host == "" {
		host = hostname
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == host {
			continue
		}
		kept = append(kept, line)
	}
	if err := os.WriteFile(file, []byte(strings.Join(kept, "\n")+"\n"), 0600); err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return knownhosts.WriteKnownHost(f, hostname, remote, key)
}

func (c *SshClient) Close() error {
	return c.conn.Close()
}

func (c *SshClient) Run(cmd string, w io.Writer) error {
	session, err := c.conn.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	session.Stdout = w
	session.Stderr = w
	err = session.Run(cmd)
	if err != nil {
		return err
	}
	return nil
}

func (c *SshClient) Upload(f *os.File, p string, m string) error {
	cp, err := scp.NewClientBySSH(c.conn)
	if err != nil {
		return err
	}
	defer cp.Close()
	return cp.CopyFromFile(context.Background(), *f, p, m)
}

func (c *SshClient) Download(f *os.File, p string, m string) error {
	cp, err := scp.NewClientBySSH(c.conn)
	if err != nil {
		return err
	}
	defer cp.Close()
	return cp.CopyFromRemote(context.Background(), f, p)
}
