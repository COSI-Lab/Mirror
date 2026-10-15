FROM python:3.14-alpine
EXPOSE 8080
RUN addgroup -S mirror-map && adduser -S mirror-map -G mirror-map
RUN mkdir /map
RUN chown mirror-map:mirror-map /map
WORKDIR /map
COPY ./requirements.txt ./requirements.txt
RUN python -m pip install -r requirements.txt
COPY ./data ./data
COPY ./src ./src
USER mirror-map
CMD ["python", "src/main.py"]