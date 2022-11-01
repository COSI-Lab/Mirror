import { setDarkState } from "./darkmode.js"

var prefersDark

var doc = document.getElementsByTagName('HTML')

var modeSwapButton = document.getElementById('darkmode_button')

if (getDarkState() === null) {
	prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
	setDarkState(prefersDark.toString())
}
else {
	prefersDark = (getDarkState() === 'true')
	setDarkState(prefersDark.toString())
}

function getDarkState() {
	return window.localStorage.getItem('mirror-dark')
}

if (prefersDark) {
	document.styleSheets.item(2).disabled = true;
	modeSwapButton.className = 'darkmode-button-active'
}

export { prefersDark }