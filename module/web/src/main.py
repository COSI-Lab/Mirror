import time
import json
import asyncio
from contextlib import asynccontextmanager
import aiohttp
from broadcaster import Broadcast
from fastapi import FastAPI, WebSocket, WebSocketDisconnect
import geoip2.database
from cachetools import TTLCache
import logging
import os

# string constants
LOKI_HOSTNAME = "loki:3100"
QUERY_ENDPOINT = "http://" + LOKI_HOSTNAME + "/loki/api/v1/query_range"
PROXY_QUERY = r'{container="proxy"} | json | __error__=`` | line_format `{{.request}}` | regexp `^GET /(?P<project>[^ /]*)` | keep project, remote_addr'
RSYNC_QUERY = r'{container="rsyncd"} |= `rsync on` | regexp `^.*? rsync on (?P<project>[^ /]*).*? from .* \((?P<remote_addr>.*?)\)` | keep project, remote_addr'
MIRROR_FILE = "./mirrors.json"

# globals
ip_cache = TTLCache(maxsize=10000, ttl=5*60)
ip_database = geoip2.database.Reader('./data/GeoLite2-City.mmdb')
broadcast = Broadcast('memory://')

# setup logger to mimic style of other components
logging.basicConfig(
    format='[%(asctime)s] [%(levelname)s] %(message)s', 
    datefmt='%m/%d/%Y %I:%M:%S %p',
    level=os.environ.get('LOGLEVEL', 'INFO').upper()
)
logging.debug("Logger Initialized")

# get project names from mirrors.json
try:
    file = open(MIRROR_FILE, 'r')
except OSError:
    logging.critical("Unable to open mirrors.json. Exiting.")
    exit(1)
else:
    try:
        # flat list of project keys for map
        project_keys = list(map(str.lower, json.load(file)['mirrors'].keys()))

        # rich map of project objects for projects page
        projects = json.load(file)['mirrors']
        for project, fields in projects.items():
            projects[project] = {key: fields[key] for key in ["alternative", "homepage", "name", "official", "page"]}

    except KeyError:
        logging.critical("Bad mirrors.json format. Exiting.")
        exit(2)
finally:
    file.close()
    logging.info("Loaded project list from mirrors.json")

def lookup_ip(ip_addr: str) -> tuple[int,int]:
    """
    Lookup an IP address in the maxmind database
    @param ip_addr The IP address to look up
    """

    try:
        match = ip_database.city(ip_addr)
        logging.info(f"Located {ip_addr} at {match.location.latitude},{match.location.longitude}")
        return match.location.latitude, match.location.longitude
    except geoip2.errors.AddressNotFoundError:
        logging.warning(f"Unable to locate {ip_addr}")
        return None, None

async def get_loki_query(session: aiohttp.ClientSession, query: str, start: int, end: int) -> dict:
    """
    Make a LogQL query to loki
    @param session 
    @param query The LogQL query
    @param start Start of time range in seconds since the unix epoch
    @param end End of time range in seconds since the unix epoch
    """

    params = {
        "query": query,
        "start": str(start),
        "end": str(end)
    }

    try:
        async with session.get(QUERY_ENDPOINT, params=params) as response:
            response.raise_for_status()
            data = await response.json()
            logging.debug("Loki query successful")
            return data
    except aiohttp.ClientError as e:
        logging.error(f"Error fetching data from {QUERY_ENDPOINT}: {e}")
    except asyncio.TimeoutError:
        logging.error(f"Request to {QUERY_ENDPOINT} timed out")

async def data_thread_task() -> None:
    """
    The thread spun up to handle a map session
    """

    logging.info("Started query thread")
    # open http session to query loki
    async with aiohttp.ClientSession() as session:
        last_updated = time.time()
        while True:
            try:
                await asyncio.sleep(3)

                update_time = time.time()
                # send queries to loki and get raw response
                queries = [
                    get_loki_query(session, RSYNC_QUERY, last_updated, update_time),
                    get_loki_query(session, PROXY_QUERY, last_updated, update_time)
                ]
                responses = await asyncio.gather(*queries)
                data = []

                # extract relevant data from response body
                for i in range(2):
                    if responses[i] is not None:
                        try:
                            data += [(entry["stream"]["remote_addr"], entry["stream"]["project"]) for entry in responses[i]["data"]["result"]]
                        except KeyError:
                            logging.warning("Hit empty query result, skipping")
                last_updated = update_time

                for ip_addr, project in data:
                    if project.lower() not in project_keys:
                        continue

                    # if we've seen this person recently, skip them
                    cache_key = ip_addr + "-" + project
                    if ip_cache.get(cache_key) is not None:
                        continue
                    ip_cache[cache_key] = -1

                    # do ip geolocation
                    lat, lon = lookup_ip(ip_addr)
                    if lat is None:
                        continue
                    msg = f"{project}\n{str(lat)}\n{str(lon)}"

                    # send to message queue
                    await broadcast.publish(channel="data", message=msg)

            except Exception:
                logging.exception(f"Data thread caught an exception")

@asynccontextmanager
async def lifespan(app: FastAPI):
    """
    Lifespan handler. Starts and cleans up after threads and the broadcast.
    """

    await broadcast.connect()
    data_task = asyncio.create_task(data_thread_task())
    yield
    data_task.cancel()
    await broadcast.disconnect()
    
app = FastAPI(lifespan=lifespan)

@app.websocket("/ws")
async def websocket_endpoint(websocket: WebSocket):
    """
    WebSocket connection handler
    """

    await websocket.accept()
    logging.debug(f"Connected to map client at {websocket.client.host}")
    try:
        async with broadcast.subscribe(channel="data") as subscriber:
            async for event in subscriber:
                await websocket.send_text(event.message)
    except WebSocketDisconnect:
        logging.debug(f"Map client {websocket.client.host} disconnected")

@app.get("/projects")
def get_projects_json():
    return projects

if __name__ == "__main__":
    import uvicorn
    logging.debug("About to start uvicorn")
    uvicorn.run(app, host="0.0.0.0", port=8080, workers=1, lifespan="on",log_config=None)
