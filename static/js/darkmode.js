import { prefersDark } from './checkdark.js'

var doc = document.getElementsByTagName('HTML')

var mainLinks = document.getElementsByTagName('MAIN')[0].getElementsByTagName('A')

console.log(mainLinks.length)

var modeSwapButton = document.getElementById('darkmode_button')

if (prefersDark) {
	modeSwapButton.className = 'darkmode-button-after'

	for (let i = 0; i < mainLinks.length; i++) {
		mainLinks[i].classList.add('main-link-dark')
	}
}
else {
	modeSwapButton.className = 'darkmode-button'

	for (let i = 0; i < mainLinks.length; i++) {
		mainLinks[i].classList.add('main-link-light')
	}
}

modeSwapButton.addEventListener('click', function () {
	if (doc[0].classList.contains('lightmode-back')) {
		doc[0].className = 'darkmode-back'
		modeSwapButton.className = 'darkmode-button-after'

		for (let i = 0; i < mainLinks.length; i++) {
			mainLinks[i].classList.remove('main-link-light')
			mainLinks[i].classList.add('main-link-dark')
		}

		setDarkState('true')
	}
	else if (doc[0].classList.contains('darkmode-back')) {
		doc[0].className = 'lightmode-back'
		modeSwapButton.className = 'darkmode-button'

		for (let i = 0; i < mainLinks.length; i++) {
			mainLinks[i].classList.remove('main-link-dark')
			mainLinks[i].classList.add('main-link-light')
		}

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