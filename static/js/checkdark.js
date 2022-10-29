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
	doc[0].className = 'darkmode-back'
	modeSwapButton.className = 'darkmode-button-after'
}

export { prefersDark }