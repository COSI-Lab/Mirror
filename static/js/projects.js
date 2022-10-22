var collapsable = document.getElementsByClassName("toc-heading"); // TODO: set id of "linux distributions", "software", and "miscellaneous" to collapsable in toc after making the following for loop work

for (let i = 0; i < collapsable.length; i++) {
  collapsable[i].addEventListener("click", function () {
    this.classList.toggle("active");
    var content = this.nextElementSibling;
    if (content.style.display === "block") {
      content.style.display = "none";
    } else {
      content.style.display = "block";
    }
  })
}