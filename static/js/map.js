// How long points are displayed (in milliseconds)
const DISPLAY_TIME = 1000 * 30;
var circles = [];

// Connects to the websocket endpoint
function connect() {
  let ws_scheme = window.location.protocol == "https:" ? "wss://" : "ws://";

  let socket = new WebSocket(ws_scheme + window.location.host + "/map/ws");
  socket.binaryType = "arraybuffer";

  socket.onopen = function (e) {
    console.log("Connected!", e);
  };
  socket.onmessage = function (message) {
    const buffer = new Uint8Array(message.data);
    console.log(buffer)

    // 8 message at 5 bytes = 40 bytes
    for (let i = 0; i < buffer.length; i += 5) {
      // First byte is the distro id
      const distro = buffer[i];
      const lat = buffer[i+1] << 8 | buffer[i+2];
      const long = buffer[i+3] << 8 | buffer[i+4];

      // Convert into x and y coordinates and put them on scale of 0-1
      const x = long / 4096;
      const y = (4096 - lat)  / 4096;

      // Add new data points to the front of the list
      circles.unshift([x, y, distro, new Date().getTime()]);

      // count hits
      distros[distro][2] += 1;
    }
  };
  socket.onclose = function (e) {
    console.log("Disconnected!", e);
  };
  socket.onerror = function (e) {
    console.log("Error!", e);

    // Try to reconnect after 5 seconds
    setTimeout(connect, 5000);
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
    let checkTime = new Date().getTime();

    ctx.globalAlpha = 1;
    ctx.drawImage(img, 0, 0, canvas.width, canvas.height);

    for (let i = 0; i < circles.length; i++) {
      let circle = circles[i];
      distros[circle[2]][3] = 1;

      // Time difference
      const delta = checkTime - circles[i][3];

      // Remove old data points
      if (delta > DISPLAY_TIME) {
        // We know all future indexes are older
        circles = circles.slice(0, i);
        break;
      }

      ctx.fillStyle = distros[circle[2]][1]; // This is ugly but it works
      ctx.beginPath();
      ctx.globalAlpha = 1 - delta / DISPLAY_TIME;
      ctx.arc(
        circle[0] * canvas.width,
        circle[1] * canvas.height,
        2.0,
        0,
        2 * Math.PI,
        false
      );
      ctx.closePath();
      ctx.fill();
    }

    ctx.beginPath();
    let incX = 0;
    let incY = 0;
    let startX = 10;
    let startY = canvas.height * 0.44;
    let maxPerColumn = (canvas.height * (0.9 - 0.44)) / 15;
    let numberOfEntries = distros.map((d) => d[3]).reduce((a, b) => a + b);

    if (numberOfEntries == 0) {
      await new Promise((r) => setTimeout(r, 15));
      continue;
    }

    let numberOfRows = Math.ceil(numberOfEntries / maxPerColumn);

    // Show rectangle
    let height = Math.min(canvas.height * (0.9 - 0.44), 15 * numberOfEntries);
    let width = numberOfRows * 130;
    ctx.globalAlpha = 1;
    ctx.fillStyle = "#282828";
    ctx.rect(5, startY - 40, width, height + 45);
    ctx.fill();

    // "Legend"
    ctx.fillStyle = "white";
    ctx.textAlign = "center";
    ctx.fillText("Legend", width * 0.5, startY - 20);

    // Print each visible distro
    ctx.font = "15px Arial";
    ctx.textAlign = "left";
    const sorted = [...distros].sort((a, b) => b[2] - a[2]);
    for (let i = 0; i < sorted.length; i++) {
      if (sorted[i][3] == 1) {
        if (startY + incY > canvas.height * 0.9) {
          incY = 0;
          incX += 130;
        }
        ctx.fillStyle = sorted[i][1];
        ctx.fillText(sorted[i][0], startX + incX, startY + incY);
        incY += 15;
        sorted[i][3] = 0;
      }
    }

    // Run around 60 fps
    await new Promise((r) => setTimeout(r, 1000/60));
  }
};
