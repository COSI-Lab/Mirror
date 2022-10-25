import { prefersDark } from './checkdark.js'

var doc = document.getElementsByTagName('HTML')

var modeSwapButton = document.getElementById('darkmode_button')

if (prefersDark) {
	modeSwapButton.className = 'darkmode-button-after'
}
else {
	modeSwapButton.className = 'darkmode-button'
}

modeSwapButton.addEventListener('click', function () {
	if (doc[0].classList.contains('lightmode-back')) {
		doc[0].className = 'darkmode-back'
		modeSwapButton.className = 'darkmode-button-after'
		setDarkState('true')
	}
	else if (doc[0].classList.contains('darkmode-back')) {
		doc[0].className = 'lightmode-back'
		modeSwapButton.className = 'darkmode-button'
		setDarkState('false')
	}
})

if (window.location.pathname == '/map') {
	var dmContainer = document.getElementById('dm_button_container')
	dmContainer.replaceChildren()
}

export function setDarkState(isDark) {
	window.localStorage.setItem('mirror-dark', isDark)
}