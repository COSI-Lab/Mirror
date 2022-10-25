var doc = document.getElementsByTagName("HTML")

var modeSwapButton = document.getElementById("darkmode_button")

modeSwapButton.addEventListener("click", function () {
	if (doc[0].classList.contains("lightmode-back")) {
		doc[0].classList.add("darkmode-back");
		doc[0].classList.remove("lightmode-back")
		modeSwapButton.classList.add("darkmode-button-after");
	}
	else if (doc[0].classList.contains("darkmode-back")) {
		doc[0].classList.add("lightmode-back");
		doc[0].classList.remove("darkmode-back")
		modeSwapButton.classList.remove("darkmode-button-after")
	}
})

if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
	modeSwapButton.classList.add("darkmode-button-after");
}

if (window.location.pathname == "/map") {
	var dmContainer = document.getElementById("dm_button_container")
	dmContainer.replaceChildren()
}