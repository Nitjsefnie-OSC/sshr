package app

import (
	"errors"
	"strings"
)

// CopyArgs builds the scp command line that copies files to h (send) or
// from it, with the same key, agent, password, jump hosts and shared
// connection as ssh, scp takes the port as -P, -p there keeps file times,
// -r is always given, so folders copy too, legacy uses the old scp
// protocol (-O), which needs no SFTP subsystem on the server
func (s *Store) CopyArgs(h *Host, send, legacy bool, sources []string, dest string) ([]string, error) {
	if len(sources) == 0 {
		return nil, errors.New("nothing to copy")
	}
	conn, err := s.ConnectArgs(h)
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, len(conn)+len(sources)+4)
	for i := 0; i < len(conn); i++ {
		if conn[i] == "-p" && i+1 < len(conn) {
			args = append(args, "-P", conn[i+1])
			i++
			continue
		}
		args = append(args, conn[i])
	}
	if legacy {
		args = append(args, "-O")
	}
	args = append(args, "-r", "--")
	if send {
		return append(append(args, sources...), remoteSpec(h, dest)), nil
	}
	for _, src := range sources {
		args = append(args, remoteSpec(h, src))
	}
	return append(args, dest), nil
}

// remoteSpec is [user@]host:path for scp, an IPv6 address goes in brackets
// so its colons are not read as the path separator, an empty path is the
// home folder
func remoteSpec(h *Host, path string) string {
	host := h.Host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if h.User != "" {
		host = h.User + "@" + host
	}
	return host + ":" + path
}
