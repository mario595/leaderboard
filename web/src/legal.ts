// Facts the legal pages and the footer share. Changing what the app collects
// means updating both pages, policyUpdated, and asking members for consent
// again (Strava API Policy 7.2).

// supportEmail is the end-user support contact (Strava API Policy 2.4), a
// forwarding alias.
export const supportEmail = 'leaderboard.oxidant225@aleeas.com'

// controllerName is the data controller named in the privacy policy.
export const controllerName = 'Mario Martinez Campos'

// policyUpdated is the date of the last change to either page, as shown.
export const policyUpdated = '12 de noviembre de 2026'

// stravaAthleteURL is the member's own profile on Strava (Policy 2.4: a clear
// link to navigate to their Strava account).
export function stravaAthleteURL(stravaId: number): string {
  return `https://www.strava.com/athletes/${stravaId}`
}
