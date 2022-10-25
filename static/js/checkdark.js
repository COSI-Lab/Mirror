var doc = document.getElementsByTagName('HTML')

var modeSwapButton = document.getElementById('darkmode_button')

var prefersDark

if (getDarkState() === null) {
	prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
}
else {
	prefersDark = (getDarkState() === 'true')
}

function getDarkState() {
	return window.localStorage.getItem('mirror-dark')
}

if (prefersDark) {
	doc[0].className = 'darkmode-back'
	modeSwapButton.className = 'darkmode-button-after'
}