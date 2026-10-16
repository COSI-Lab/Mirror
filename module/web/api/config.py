import logging
import json

# constants
LOKI_HOSTNAME = "loki:3100"
QUERY_ENDPOINT = "http://" + LOKI_HOSTNAME + "/loki/api/v1/query_range"
PROXY_QUERY = r'{container="proxy"} | json | __error__=`` | line_format `{{.request}}` | regexp `^GET /(?P<project>[^ /]*)` | keep project, remote_addr'
RSYNC_QUERY = r'{container="rsyncd"} |= `rsync on` | regexp `^.*? rsync on (?P<project>[^ /]*).*? from .* \((?P<remote_addr>.*?)\)` | keep project, remote_addr'
MIRROR_FILE = "/Users/anthonyhalliday/Documents/mirror/config/mirrors.json"

# get project names from mirrors.json
try:
    file = open(MIRROR_FILE, 'r')
except OSError:
    logging.critical("Unable to open mirrors.json. Exiting.")
    exit(1)
else:
    try:
        projects = json.load(file)['mirrors']
        project_keys = list(map(str.lower, projects.keys()))
        for project, fields in projects.items():
            projects[project] = {key: fields.get(key) for key in ["alternative", "homepage", "name", "official", "page"]}

    except KeyError:
        logging.critical("Bad mirrors.json format. Exiting.")
        exit(2)
finally:
    file.close()
    logging.info("Loaded project list from mirrors.json")