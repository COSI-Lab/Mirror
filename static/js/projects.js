var collapsible = document.getElementsByClassName("toc-heading");

for (let i = 0; i < collapsible.length; i++) {
  collapsible[i].addEventListener("click", function () {
    this.classList.toggle("toc-heading-extended");
    var content = this.nextElementSibling;
    if (content.style.display === "block") {
      content.style.display = "none";
    }
    else {
      content.style.display = "block";
    }
  })
}

var imageTemplates = document.getElementsByTagName("TEMPLATE")
var imageContainers = document.getElementsByClassName("icon-container")
var imagesLoaded = false

if (window.innerWidth > 800 && !imagesLoaded) {
  for (let index = 0; index < imageTemplates.length; index++) {
    var image = imageTemplates[index].content.cloneNode(true)
    imageContainers[index].appendChild(image)
  }
  imagesLoaded = true
}
else {
  window.addEventListener('resize', function () {
    if (window.innerWidth > 800 && !imagesLoaded) {
      for (let index = 0; index < imageTemplates.length; index++) {
        var image = imageTemplates[index].content.cloneNode(true)
        imageContainers[index].appendChild(image)
      }
      imagesLoaded = true
    }
  })
}