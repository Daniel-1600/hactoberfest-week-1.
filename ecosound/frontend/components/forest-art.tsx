export default function ForestArt() {
  return <svg viewBox="0 0 640 440" role="img" aria-label="Illustration of a bird singing among moonlit trees" className="forest-art">
    <defs>
      <linearGradient id="sky" x2="0" y2="1"><stop stopColor="#1a1a2e"/><stop offset="1" stopColor="#16213e"/></linearGradient>
      <linearGradient id="hill" x2="0" y2="1"><stop stopColor="#1a3a28"/><stop offset="1" stopColor="#0d1f15"/></linearGradient>
      <pattern id="grain" width="7" height="7" patternUnits="userSpaceOnUse"><circle cx="1" cy="1" r=".6" fill="#3ecf8e" opacity=".04"/></pattern>
    </defs>
    <rect width="640" height="440" rx="24" fill="url(#sky)"/>
    <circle cx="463" cy="122" r="65" fill="#2a2a3e" opacity=".6"/>
    <circle cx="463" cy="122" r="50" fill="#c8cad0" opacity=".15"/>
    <path d="M0 283Q120 190 263 272T640 244V440H0Z" fill="#162b1e"/>
    <path d="M0 350Q114 245 310 326T640 290V440H0Z" fill="url(#hill)"/>
    <path d="M340 440Q378 360 520 319Q425 375 446 440" fill="#1a2e1f"/>
    <path d="M57 440L93 109L127 440M547 440L578 59L622 440" fill="#0f2618"/>
    <path d="M21 250L98 57L173 250Z" fill="#133020"/><path d="M0 317L97 109L191 317Z" fill="#0e2418"/>
    <path d="M497 195L573 5L640 195Z" fill="#1a3a25"/><path d="M480 279L579 80L640 250V279Z" fill="#123020"/>
    <path d="M203 440Q222 290 204 209" stroke="#2a1f15" strokeWidth="13" fill="none"/>
    <path d="M205 290Q282 264 343 251" stroke="#2a1f15" strokeWidth="11" strokeLinecap="round"/>
    <ellipse cx="198" cy="182" rx="84" ry="94" fill="#1a4030"/><ellipse cx="148" cy="223" rx="52" ry="61" fill="#1a4030"/><ellipse cx="246" cy="223" rx="67" ry="57" fill="#1a4030"/>
    <path d="M198 231Q180 188 172 177M198 211L219 169" stroke="#16352a" strokeWidth="3" fill="none"/>
    <path d="M296 247L272 266L272 239" fill="#6d4020"/>
    <ellipse cx="317" cy="222" rx="34" ry="31" transform="rotate(-15 317 222)" fill="#8a6535"/>
    <path d="M292 210Q302 250 335 236Q345 217 313 207" fill="#6d4020"/>
    <circle cx="342" cy="201" r="22" fill="#8a6535"/>
    <path d="M351 202L371 209L352 215" fill="#2a1f15"/>
    <circle cx="350" cy="195" r="3.4" fill="#ededed"/>
    <path d="M322 249L321 260M331 248L330 258" stroke="#2a1f15" strokeWidth="3"/>
    <path d="M382 183Q395 194 385 207M398 171Q420 192 404 216" fill="none" stroke="#3ecf8e" strokeWidth="3" strokeLinecap="round" opacity=".5"/>
    <path d="M442 93Q451 83 461 93M484 175Q493 165 503 175" stroke="#3ecf8e" strokeWidth="3" fill="none" strokeLinecap="round" opacity=".3"/>
    <path d="M20 440Q30 360 71 363Q79 396 49 421Q97 350 119 372Q130 401 91 435M500 440Q514 383 540 383Q546 415 523 434Q571 380 592 407Q591 433 559 440" fill="#0a1f12"/>
    <circle cx="120" cy="80" r="2" fill="#ededed" opacity=".4"/>
    <circle cx="280" cy="50" r="1.5" fill="#ededed" opacity=".3"/>
    <circle cx="380" cy="35" r="1" fill="#ededed" opacity=".5"/>
    <circle cx="530" cy="65" r="1.8" fill="#ededed" opacity=".35"/>
    <circle cx="200" cy="30" r="1.2" fill="#ededed" opacity=".25"/>
    <circle cx="80" cy="55" r="1" fill="#ededed" opacity=".3"/>
    <circle cx="560" cy="40" r="1.5" fill="#ededed" opacity=".4"/>
    <rect width="640" height="440" rx="24" fill="url(#grain)"/>
  </svg>;
}
