<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
  <xsl:template match="/">
    <main>
    <div class="card_container">
      <xsl:for-each select="list/directory">
        <article class="card">
            <img class="mirror_icon">
              <xsl:attribute name="src">
                <xsl:text>/static/img/projects/</xsl:text>
                <xsl:value-of select="./text()"/>
              </xsl:attribute>
            </img>
            <a>
              <xsl:attribute name="href">
              <xsl:value-of select="./text()"/>
              <xsl:text>/</xsl:text>
              </xsl:attribute>
              <header>
                <h2>
                  <xsl:value-of select="./text()"/>
                </h2>
              </header>
            </a>
        </article>
      </xsl:for-each>
    </div>
    </main>
  </xsl:template>
</xsl:stylesheet>