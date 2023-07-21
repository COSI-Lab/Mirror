document.addEventListener('keyup', (e) => {
	const name = e.key;

	if (name == 'Insert') {
		var header = document.getElementsByTagName('header')[0];

		if (header.style.display === '') 
			header.style.display = 'none';

		else if (header.style.display === 'none')
			header.style.display = '';
	}
})