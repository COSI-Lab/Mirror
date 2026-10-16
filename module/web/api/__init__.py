from fastapi import FastAPI
import logging
import os

from .router import map, projects

# setup logger to mimic style of other components
logging.basicConfig(
    format='[%(asctime)s] [%(levelname)s] %(message)s', 
    datefmt='%m/%d/%Y %I:%M:%S %p',
    level=os.environ.get('LOGLEVEL', 'INFO').upper()
)
logging.debug("Logger Initialized")

app = FastAPI(lifespan=map.lifespan,root_path="/api")
app.include_router(map.router)
app.include_router(map.router)

if __name__ == "__main__":
    import uvicorn
    logging.debug("About to start uvicorn")
    uvicorn.run(app, host="0.0.0.0", port=8080, workers=1, lifespan="on", log_config=None)
