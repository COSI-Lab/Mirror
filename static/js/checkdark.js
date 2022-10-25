var doc = document.getElementsByTagName("HTML")

var modeSwapButton = document.getElementById("darkmode_button")

if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
	doc[0].classList.add("darkmode-back");
	doc[0].classList.remove("lightmode-back")
	modeSwapButton.classList.add("darkmode-button-after");
}