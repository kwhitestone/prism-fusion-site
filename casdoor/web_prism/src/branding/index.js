// Centralized branding configuration for PrismAccount
// To rebrand, only modify files in this folder (branding/).
//
// Files:
//   index.js    - All brand strings, URLs, and logo imports
//   logo.svg    - The brand logo (SVG format)
//
// After changing values here, the entire UI updates automatically.

import logo from "./logo.svg";

// ============================================================
// Brand Identity
// ============================================================

/** Brand name shown in UI, page titles, email templates, etc. */
export const BrandName = "PrismAccount";

/** Short tagline / meta description */
export const BrandDescription =
  "PrismAccount - An Identity and Access Management (IAM) / Single-Sign-On (SSO) platform";

/** Organization name used in email templates */
export const BrandOrganization = "Whitestone";

// ============================================================
// Logos & Icons
// ============================================================

/** Imported SVG logo (works in both light & dark themes) */
export const BrandLogo = logo;

/** Favicon URL — served from public/img/ */
export const BrandFavicon = "/img/logo.svg";

/** Logo used in HTML email templates (must be an absolute URL for email clients).
 *  Set to empty string "" to omit logo from emails. */
export const BrandEmailLogoUrl = "";

// ============================================================
// URLs
// ============================================================

/** Brand home page */
export const BrandHomepage = "https://whitestone.top";

/** Documentation base URL (used for provider doc links, etc.) */
export const BrandDocsUrl = "https://whitestone.fun/docs";

// ============================================================
// Email Templates
// ============================================================

/** Default email sender display name */
export const BrandEmailSubjectPrefix = `${BrandName} Verification Code`;

/** Email footer disclaimer */
export const BrandEmailFooter = `${BrandName} is operated by ${BrandOrganization}. For more info please refer to <a href="${BrandHomepage}">${BrandHomepage}</a>`;

// ============================================================
// ICP / Beian (China compliance)
// ============================================================

export const BeianIcp = "闽ICP备2026001798号-1";
export const BeianIcpUrl = "https://beian.miit.gov.cn/";
export const BeianGongan = "闽公网安备35011102351136号";
export const BeianGonganCode = "35011102351136";
export const BeianGonganUrl = `https://beian.mps.gov.cn/#/query/webSearch?code=${BeianGonganCode}`;
