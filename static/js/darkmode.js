import { prefersDark } from './checkdark.js'

var doc = document.getElementsByTagName('HTML')

var modeSwapButton = document.getElementById('darkmode_button')

if (prefersDark) {
	modeSwapButton.className = 'darkmode-button-active'
}
else {
	modeSwapButton.className = 'darkmode-button-inactive'
}

modeSwapButton.addEventListener('click', function () {
	if (!document.styleSheets.item(2).disabled) {
		document.styleSheets.item(2).disabled = true;
		modeSwapButton.className = 'darkmode-button-active'
		setDarkState('true')
	}
	else if (document.styleSheets.item(2).disabled) {
		document.styleSheets.item(2).disabled = false;
		modeSwapButton.className = 'darkmode-button-inactive'
		setDarkState('false')
	}
})

if (window.location.pathname === '/map') {
	var dmContainer = document.getElementById('dm_button_container')
	dmContainer.replaceChildren()
}

export function setDarkState(isDark) {
	window.localStorage.setItem('mirror-dark', isDark)
}