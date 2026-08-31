const MAP_LEGEND = document.getElementById("map_legend");
const MAX_CIRCLES = 1000;
const CANVAS = document.getElementById("canvas");
const IMG = document.getElementById("map");
const ctx = CANVAS.getContext("2d");
var circles = [];
var legend_entries = new Map();

/**
 * Gets the hue and luminance for a project's color from its name
 * @param {String} project Project name to hash to get hue
 * @returns {[Number]} Hue and Luminance for this project
 */
function getColor(project) {
    let hash = 0;
    for (let i = 0, len = project.length; i < len; i++) {
        let chr = project.charCodeAt(i);
        hash = (hash << 5) - hash + chr;
        hash |= 0;
    }
    hash = Math.abs(hash);
    return [hash % 360, 50 + (hash % 2) * 25];
}

/**
 * Renders legend entries
 * @param {Map<String, [Number]>} legend_entries 
 */
async function renderLegend(legend_entries) {
    legend_html = "";
    legend_entries.forEach((v, k) => {
        legend_html += `<span style="color: hsl(${v[0]}, 100%, ${v[1]}%)">${k}</span><br>`;
    });
    MAP_LEGEND.innerHTML = legend_html;
}

/**
 * Connects to the websocket backend to receive events
 */
function registerWebsocket() {
    socket = new WebSocket((window.location.protocol === "https:" ? "wss://" : "ws://") + window.location.host + "/ws");

    // Define an event listener for the 'open' event
    socket.onopen = function() {
        console.log("Connected to the WebSocket server");
    };

    // Define an event listener for the 'message' event
    socket.onmessage = function(event) {
        console.log("Received message: " + event.data);
        var data = event.data.split("\n");

        // Update legend
        var color = getColor(data[0]);
        legend_entries.set(data[0], color);
        renderLegend(legend_entries);

        // Add circle
        data[1] = 1 - ((parseFloat(data[1]) + 90) / 180);
        data[2] = (parseFloat(data[2]) + 180) / 360;
        data.push(color[0]);
        data.push(color[1]);
        circles.push(data);

        if(circles.length > MAX_CIRCLES) { circles.shift(); }
    };

    // Define an event listener for the 'close' event
    socket.onclose = function() {
        console.log("Disconnected! Attempting to reconnect...");
        setTimeout(registerWebsocket, 1000);
    };

    // Define an event listener for the 'error' event
    socket.onerror = function(error) {
        console.log("Error occurred: " + error);
    };
}

/* --- Global event handlers --- */

// Event listener for window size change
window.onresize = function(){
    CANVAS.width = window.innerWidth;
    CANVAS.height = window.innerHeight;
};

/* --- On page load --- */

// Connect to backend
registerWebsocket();

// Size canvas properly
window.onresize();

// Draw loop
setInterval(() => {
    ctx.drawImage(IMG, 0, 0, CANVAS.width, CANVAS.height);

    circles.forEach((circle) => {
        ctx.fillStyle = `hsl(${circle[3]}, 100%, ${circle[4]}%)`;
        
        // Draw the circle
        ctx.beginPath();
        ctx.arc(
            circle[2] * CANVAS.width,
            circle[1] * CANVAS.height,
            2.0,
            0,
            2 * Math.PI,
            false
        );
        ctx.closePath();
        ctx.fill();
    })

}, 500); // Draw every 500 ms
