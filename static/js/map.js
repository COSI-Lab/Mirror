// How long points are displayed (in milliseconds)
const displayTimeSeconds = 900;
const MILLISECONDS_PER_SECOND = 1000
const DISPLAY_TIME = MILLISECONDS_PER_SECOND * displayTimeSeconds;
var circles = [];

// Circle class
class Circle {
  constructor(x, y, distro, time) {
    this.x = x;
    this.y = y;
    this.distro = distro;
    this.time = time;
  }
}

// Connects to the websocket
function connect() {
  let ws_scheme = window.location.protocol === "https:" ? "wss://" : "ws://";

  let socket = new WebSocket(ws_scheme + window.location.host + "/ws");
  socket.binaryType = "arraybuffer";

  socket.onmessage = async function (message) {
    const buffer = new Uint8Array(message.data);
    const time = new Date().getTime();

    // Messages are sent in large groups, where every 5 bytes is a new data point
    for (let i = 0; i < buffer.length; i += 5) {
      // u8 distro id
      const distro = buffer[i];
      // u16 lat
      const lat = buffer[i + 1] << 8 | buffer[i + 2];
      // u16 long
      const long = buffer[i + 3] << 8 | buffer[i + 4];

      // Convert lat long into (x, y) coordinates for the map and scale them between 0-1
      const x = long / 4096;
      const y = (4096 - lat) / 4096;

      // Add new data points to the end of the array
      circles.push(new Circle(x, y, distro, time));

      // count hits
      distros[distro][2] += 1;
    }
  };

  socket.onclose = function (e) {
    console.log("Disconnected from server, reconnecting in 5 seconds...");
    setTimeout(connect, 5000);
  };

  socket.onerror = function (e) {
    console.error(e);
  };

  return socket;
}

window.onload = async function () {
  connect();

  const canvas = document.getElementById("myCanvas");
  const ctx = canvas.getContext("2d");
  const img = document.getElementById("map");

  window.onresize = function () {
    canvas.width = window.innerWidth;

    // Height is viewport height - "#header" height 
    canvas.height = window.innerHeight - document.getElementById("header").clientHeight;
  };

  window.onresize();

  while (true) {
    // Clear the canvas
    ctx.globalAlpha = 1;
    ctx.drawImage(img, 0, 0, canvas.width, canvas.height);

    // Remove old data points
    const time = new Date().getTime();

    // Find the index of the first data point that is too old
    let index = circles.findIndex((c) => time - c[3] > DISPLAY_TIME);
    if (index != -1) {
      circles.splice(0, index);
    }

    // Draw each circle
    for (const circle of circles) {
      const color = distros[circle.distro][1];
      ctx.fillStyle = color;
      ctx.beginPath();
      ctx.globalAlpha = 1 - ((time - circle.time) / DISPLAY_TIME);
      ctx.arc(
        circle.x * canvas.width,
        circle.y * canvas.height,
        2.0, // Radius
        0,
        2 * Math.PI, // Full circle
        false
      );
      ctx.closePath();
      ctx.fill();
    }

    // Draw the legend
    // TODO: Putting this on the canvas and doesn't scale well
    ctx.beginPath();
    let incX = 0;
    let incY = 0;
    let startX = 10;
    let startY = canvas.height * 0.30;

    let legendEntries = distros.map((d) => d[3]).reduce((a, b) => a + b);
    if (legendEntries == 0) {
      await new Promise((r) => setTimeout(r, 15));
      continue;
    }
    console.log(legendEntries)

    // Show rectangle
    let height = 15 * legendEntries;
    let width = 135;
    ctx.globalAlpha = 1;
    ctx.fillStyle = "#383838";
    ctx.rect(5, startY - 40, width, height + 45);
    ctx.fill();

    // "Legend"
    ctx.fillStyle = "white";
    ctx.textAlign = "center";
    ctx.fillText("Legend", width * 0.5, startY - 20);

    // Print each visible distro
    ctx.font = "15px Arial";
    ctx.textAlign = "left";
    const sortedLegend = [...distros].sort((a, b) => b[2] - a[2]);
    for (let i = 0; i < sortedLegend.length; i++) {
      if (sortedLegend[i][3] == 1) {
        if (startY + incY > canvas.height * 0.9) {
          incY = 0;
          incX += 130;
        }
        ctx.fillStyle = sortedLegend[i][1];
        ctx.fillText(sortedLegend[i][0], startX + incX, startY + incY);
        incY += 15;
        sortedLegend[i][3] = 0;
      }
    }

    const framesPerSecond = 5
    await new Promise((handler) => setTimeout(handler, MILLISECONDS_PER_SECOND / framesPerSecond));
  }
};
