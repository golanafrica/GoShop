# 🔒 Security Policy / Politique de Sécurité

**Version :** 1.0
**Last updated / Dernière mise à jour :** 13 août 2026
**Maintained by / Maintenu par :** GoShop Security Team

---

## 📋 Table of Contents / Table des Matières

1. [Reporting a Vulnerability / Signaler une Vulnérabilité](#-reporting-a-vulnerability--signaler-une-vulnérabilité)
2. [Response Timeline / Délais de Réponse](#-response-timeline--délais-de-réponse)
3. [Supported Versions / Versions Supportées](#-supported-versions--versions-supportées)
4. [Disclosure Policy / Politique de Divulgation](#-disclosure-policy--politique-de-divulgation)
5. [Safe Harbor / Port Sûr](#-safe-harbor--port-sûr)
6. [Scope / Périmètre](#-scope--périmètre)
7. [Hall of Fame / Mur de la Renommée](#-hall-of-fame--mur-de-la-renommée)

---

## 🚨 Reporting a Vulnerability / Signaler une Vulnérabilité

### English

**⚠️ DO NOT open a public GitHub issue for security vulnerabilities.**

If you discover a security vulnerability in GoShop, please report it responsibly:

1. **Email:** Send a detailed report to **security@golanafrica.com**
2. **Encryption (recommended):** Use our PGP key (available on request) for sensitive reports
3. **Include in your report:**
   - Description of the vulnerability
   - Steps to reproduce (PoC code if possible)
   - Potential impact assessment
   - Suggested fix (if you have one)
   - Your contact information for follow-up

### Français

**⚠️ N'ouvrez PAS d'issue GitHub publique pour les vulnérabilités de sécurité.**

Si vous découvrez une vulnérabilité de sécurité dans GoShop, veuillez la signaler de manière responsable :

1. **Email :** Envoyez un rapport détaillé à **security@golanafrica.com**
2. **Chiffrement (recommandé) :** Utilisez notre clé PGP (disponible sur demande) pour les rapports sensibles
3. **Incluez dans votre rapport :**
   - Description de la vulnérabilité
   - Étapes pour reproduire (code PoC si possible)
   - Évaluation de l'impact potentiel
   - Correctif suggéré (si vous en avez un)
   - Vos coordonnées pour le suivi

---

## ⏱️ Response Timeline / Délais de Réponse

We commit to the following response times / Nous nous engageons sur les délais suivants :

| Phase | Timeline / Délai | Description |
|-------|------------------|-------------|
| **Acknowledgment / Accusé de réception** | 48 hours / 48 heures | We confirm receipt of your report / Nous confirmons la réception de votre rapport |
| **Validation** | 7 days / 7 jours | We verify the vulnerability and assess severity / Nous vérifions la vulnérabilité et évaluons la sévérité |
| **Fix Development / Développement du correctif** | 30-90 days / 30-90 jours | We develop and test a fix based on severity / Nous développons et testons un correctif selon la sévérité |
| **Disclosure / Divulgation** | 90 days / 90 jours | Public disclosure after fix is released / Divulgation publique après publication du correctif |

**Severity Levels / Niveaux de Sévérité :**

- **Critical / Critique** (CVSS 9.0-10.0): Fix within 7 days / Correctif en 7 jours
- **High / Élevé** (CVSS 7.0-8.9): Fix within 30 days / Correctif en 30 jours
- **Medium / Moyen** (CVSS 4.0-6.9): Fix within 60 days / Correctif en 60 jours
- **Low / Faible** (CVSS 0.1-3.9): Fix within 90 days / Correctif en 90 jours

---

## 📦 Supported Versions / Versions Supportées

We provide security updates for the following versions / Nous fournissons des mises à jour de sécurité pour les versions suivantes :

| Version | Supported / Supportée | End of Life / Fin de Support |
|---------|----------------------|------------------------------|
| 5.x | ✅ Yes / Oui | N/A |
| 4.x | ✅ Yes / Oui | December 31, 2026 / 31 décembre 2026 |
| 3.x | ❌ No / Non | June 30, 2026 / 30 juin 2026 |
| < 3.0 | ❌ No / Non | Already EOL / Déjà en fin de vie |

**Policy / Politique :**
- We support the **latest major version (N)** and the **previous major version (N-1)**
- Security patches are backported to N-1 for 6 months after N release
- Critical vulnerabilities may receive emergency patches for older versions at our discretion

---

## 📢 Disclosure Policy / Politique de Divulgation

### Coordinated Disclosure / Divulgation Coordonnée

We follow a **90-day coordinated disclosure** process:

1. **Day 0:** Vulnerability reported to us
2. **Day 1-7:** We validate and assess the vulnerability
3. **Day 8-30:** We develop and test a fix
4. **Day 31-60:** We release the fix and notify affected users
5. **Day 61-90:** Embargo period before public disclosure
6. **Day 90:** Public disclosure (with your permission)

### Researcher Guidelines / Directives pour les Chercheurs

**DO / À FAIRE :**
- ✅ Report vulnerabilities privately first
- ✅ Give us reasonable time to fix before public disclosure
- ✅ Avoid accessing/modifying user data during testing
- ✅ Document your findings thoroughly

**DON'T / À NE PAS FAIRE :**
- ❌ Exploit vulnerabilities beyond what's needed for PoC
- ❌ Disclose vulnerabilities publicly before the embargo period ends
- ❌ Perform DoS attacks or resource exhaustion testing
- ❌ Social engineering or phishing attacks against users/staff

---

## 🛡️ Safe Harbor / Port Sûr

We consider security research conducted in accordance with this policy to be **authorized** and will not pursue legal action against researchers who:

- Act in good faith and avoid privacy violations
- Do not degrade system performance (no DoS)
- Report vulnerabilities within a reasonable timeframe
- Do not exploit vulnerabilities for malicious purposes

**This safe harbor applies to:**
- Testing on our official environments (staging, production)
- Research on code available in our public repositories
- Vulnerabilities discovered through authorized testing methods

---

## 🎯 Scope / Périmètre

### In Scope / Dans le Périmètre

- ✅ GoShop API (`api.goshop.com` and subdomains)
- ✅ GoShop web applications (`app.goshop.com`, `admin.goshop.com`)
- ✅ GoShop mobile apps (iOS, Android)
- ✅ GoShop public repositories (GitHub: `golanafrica/GoShop`)
- ✅ Authentication and authorization mechanisms
- ✅ Payment processing integrations (Orange Money, Moov Money, Wave, Yenga Pay)
- ✅ Multi-tenant isolation and data segregation
- ✅ WebSocket real-time notifications
- ✅ KYC/AML compliance workflows

### Out of Scope / Hors Périmètre

- ❌ Third-party services we integrate with (report to them directly)
- ❌ Denial of Service (DoS) attacks
- ❌ Social engineering or phishing
- ❌ Physical security attacks
- ❌ Vulnerabilities in outdated browsers or OS versions
- ❌ Issues already known or reported by another researcher
- ❌ Vulnerabilities requiring physical access to user devices

---

## 🏆 Hall of Fame / Mur de la Renommée

We publicly acknowledge security researchers who responsibly disclose vulnerabilities (with their permission).

**Recognition Criteria / Critères de Reconnaissance :**
- Valid vulnerability with CVSS score ≥ 4.0
- Clear reproduction steps provided
- Cooperative during the disclosure process
- Agreement to be publicly acknowledged

**Recent Contributors / Contributeurs Récents :**

| Researcher / Chercheur | Vulnerability Type / Type de Vulnérabilité | Date |
|------------------------|-------------------------------------------|------|
| *(Your name here)* | *(Your discovery)* | 2026 |

*Want to be listed here? Let us know when you submit your report!*

---

## 📞 Contact / Contact

### Primary Contact / Contact Principal

**Email:** security@golanafrica.com
**Response Time:** 48 hours / 48 heures
**Languages:** English, French / Anglais, Français

### Emergency Contact / Contact d'Urgence

For critical vulnerabilities requiring immediate attention / Pour les vulnérabilités critiques nécessitant une attention immédiate :

**Email:** security@golanafrica.com (subject: `[CRITICAL]`)
**PGP Key:** Available on request / Disponible sur demande

---

## 📄 Legal / Juridique

This security policy is governed by the laws of Burkina Faso. Any disputes shall be resolved through amicable negotiation or, failing that, through the competent courts of Ouagadougou.

Cette politique de sécurité est régie par les lois du Burkina Faso. Tout litige sera résolu par négociation amiable ou, à défaut, par les tribunaux compétents de Ouagadougou.

---

## 🔄 Updates / Mises à Jour

This policy may be updated from time to time. Significant changes will be announced via:
- GitHub repository updates
- Email to registered security researchers
- GoShop security blog

Cette politique peut être mise à jour périodiquement. Les changements importants seront annoncés via :
- Mises à jour du dépôt GitHub
- Email aux chercheurs en sécurité enregistrés
- Blog sécurité GoShop

---

**Thank you for helping keep GoShop secure! / Merci de contribuer à la sécurité de GoShop !**

🌍 **Built with security in mind for Africa / Construit avec la sécurité à l'esprit pour l'Afrique**