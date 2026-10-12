<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
    <xsl:output method="html" indent="yes"/>
    <xsl:template match="/">
        <!-- BEFORE -->
        <html lang="en">
        <head>
            <meta charset="UTF-8"/>
            <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
            <title>mirror.clarkson.edu</title>
            <link rel="stylesheet" href="./static/css/style.css"/>
            <link rel="stylesheet" href="./static/fa/css/solid.min.css"/>
            <link rel="stylesheet" href="./static/fa/css/fontawesome.min.css"/>
            <link rel="icon" href="./static/img/favicon.ico"/>
            <script src="./static/js/jquery/jquery-3.7.1.min.js"/>
        </head>

        <body>
            <nav>
                <span class="expand_button" onclick="$('nav').toggleClass('expanded')"><i class="fa-solid fa-bars"></i></span>
                <a href="/home" id="navbar_home">Home</a>
                <a href="/projects" id="navbar_projects">Projects</a>
                <a href="/about" id="navbar_about">About</a>
                <a href="/map" id="navbar_map">Map</a>
            </nav>

            <div class="navbar_spacer"></div>

        <!-- Original Content -->
            <xsl:copy-of select="/"/>

        <!-- AFTER -->
            <footer>
                <div class="content">
                    Hosted by the
                    <a class="footer_link" href="https://cosi.clarkson.edu">Clarkson Open Source Institute</a>.
                    <br/>
                    <br/>
                    Mirror is Free and Open Source.
                    <a class="footer_link" href="https://git.cosi.clarkson.edu/mirror/mirror">View on GitLab</a>
                </div>
            </footer>
        </body>
        </html>
    </xsl:template>
</xsl:stylesheet>