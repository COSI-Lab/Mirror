import subprocess
from time import sleep
import os

# URL of bucket root (ex. "https://kicad-downloads.s3.cern.ch")
BUCKET_URL = "https://kicad-downloads.s3.cern.ch"

# Path of download root (ex. "/storage/kicad/")
DOWNLOAD_PREFIX = "/storage/kicad/"

def runCommandWithOutput(args: list[str]) -> str:
    return subprocess.run(args, stdout=subprocess.PIPE).stdout.decode()

def runCommand(args: list[str]) -> str:
    subprocess.run(args)

def isFile(path: str):
    return not (path.endswith("/") or path.endswith("index.html")
                or path.endswith("list.js") or path.endswith("favicon.ico"))

def isValue(line: str):
    return not line.startswith("<")

def getNextMarker(xml: str):
    lines: list[str] = xml.replace("<NextMarker>", "\n").replace("</NextMarker>", "\n").splitlines()
    markers: list[str] = list(filter(lambda x: isValue(x), lines))
    if(len(markers) > 0):
        return markers[0]
    return ""

def getFilePaths(bucket_url: str) -> list[str]:
    paths: list[str] = []
    suffix: str = ""
    i: int = 0
    while(True):
        print("----- Fetching block {} of index... -----".format(i))
        full_url: str = "{}{}".format(bucket_url, suffix)
        xml: str = runCommandWithOutput(["curl", "-s", full_url])
        lines: list[str] = xml.replace("<Key>", "\n").replace("</Key>", "\n").splitlines()
        paths.extend(list(filter(lambda x: isValue(x) and isFile(x), lines)))
        next_marker: str = getNextMarker(xml)
        if(next_marker == ""):
            print("----- Done fetching index. -----")
            break
        suffix = "?marker={}".format(next_marker)
        i += 1
        sleep(0.25)
    return paths

def downloadFile(bucket_url: str, dl_prefix: str, path: str):
    full_url: str = bucket_url + "/" + path
    if(os.path.exists("{}{}".format(dl_prefix, path))):
        print("Skipping this file (already exists).")
    else:
        runCommand(["wget", "-c", "-nH", "-x", "-P", dl_prefix, full_url])

def main():
    print("----- Fetching index... -----")
    paths: list[str] = getFilePaths(BUCKET_URL)
    n_paths: int = len(paths)
    print("----- Need to get {} files. -----".format(n_paths))
    i: int = 1
    for path in paths:
        print("----- Downloading file {} of {} ({})... -----".format(i, n_paths, path))
        downloadFile(BUCKET_URL, DOWNLOAD_PREFIX, path)
        i += 1

if(__name__ == "__main__"):
    try:
        main()
    except(KeyboardInterrupt):
        print("CTRL-C")
        pass
