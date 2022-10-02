// TODO: Make a working collapsable

var collapsable = document.getElementById("collapsable"); // TODO: set id of "linux distributions", "software", and "miscellaneous" to collapsable in toc after making the following for loop work

for (let i = 0; i < collapsable.length; i++) {
    collapsable[i].addEventListener("click", function() {
        this.classList.toggle("active");

    })
    
}