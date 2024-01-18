package config

import (
	"io"
	"text/template"
)

// CreateRSCYNDConfig writes a rsyncd.conf file to the given writer based on the Config struct
func (config *File) CreateRSCYNDConfig(w io.Writer) error {
	tmpl := `# This is a generated file. Do not edit manually.
	uid = nobody
	gid = nogroup
	use chroot = yes
	max connections = 0
	pid file = /var/run/rsyncd.pid
	motd file = /etc/rsyncd.motd
	log file = /var/log/rsyncd.log
	log format = %t %o %a %m %f %b
	dont compress = *.gz *.tgz *.zip *.z *.Z *.rpm *.deb *.bz2 *.tbz2 *.xz *.txz *.rar
	refuse options = checksum delete
	{{ range . }}
	[{{ .Short }}]
		comment = {{ .Name }}
		path = /storage/{{ .Short }}
		exclude = lost+found/
		read only = true
		ignore nonreadable = yes{{ end }}
	`

	var filteredProjects []*Project
	for _, project := range config.Projects {
		if project.PublicRsync {
			filteredProjects = append(filteredProjects, project)
		}
	}

	t := template.Must(template.New("rsyncd.conf").Parse(tmpl))
	err := t.Execute(w, filteredProjects)
	if err != nil {
		return err
	}

	return nil
}
