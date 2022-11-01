const navMenu = document.getElementById("nav_menu")
const navToggleButton = document.getElementById("nav_collapse_button")

navToggleButton.addEventListener('click', function () {
	navToggleButton.classList.toggle("navbutton-expanded")
	if (navMenu.classList.contains("navmenu-collapsed")) {
		navMenu.classList.add("navmenu-expanded")
		navMenu.classList.remove("navmenu-collapsed")
	}
	else {
		navMenu.classList.add("navmenu-collapsed")
		navMenu.classList.remove("navmenu-expanded")
	}
})