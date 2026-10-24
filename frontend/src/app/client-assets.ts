import {SpireApi}  from "@/app/api/spire-api";
import EqAssets   from "@/app/eq-assets/eq-assets";

const LINK_ID = "client-assets-css"

/**
 * The client's own icons and item model previews, imported on the Icons &
 * Models page. A generated stylesheet overrides the stock asset pack's
 * classes, so it must load after the stock stylesheets.
 */
export class ClientAssets {
  static async loadCss() {
    let version = 0
    try {
      const r = await SpireApi.v1().get("client-assets/version")
      version = r.data.version || 0
    } catch (e) {
      return
    }

    const href = SpireApi.getBasePath() + "/api/v1/client-assets/file/client-assets.css?v=" + version
    let link   = document.getElementById(LINK_ID) as HTMLLinkElement
    if (link) {
      link.href = href
      return
    }

    link      = document.createElement("link")
    link.id   = LINK_ID
    link.rel  = "stylesheet"
    link.href = href
    document.head.appendChild(link)
  }

  // after an import: new icons and models show without a page reload
  static async refresh() {
    EqAssets.itemIcons          = []
    EqAssets.itemModelFileNames = []
    await ClientAssets.loadCss()
  }
}
