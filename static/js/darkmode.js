var doc = document.getElementsByTagName('HTML')

var modeSwapButton = document.getElementById('darkmode_button')

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

if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
	modeSwapButton.className = 'darkmode-button-after'
}

if (window.location.pathname == '/map') {
	var dmContainer = document.getElementById('dm_button_container')
	dmContainer.replaceChildren()
}

var setDarkState = function (isDark) {
	console.log(isDark)
	localStorage.setItem('mirror-dark', isDark)
}