// Package e2e holds the end-to-end test of the core (SPEC §16): the sample
// cassette is imported, a lesson is played to its end on the fake sink, a
// submission is opened, a photo is uploaded through the real phone server on
// 127.0.0.1, the fake provider grades it, and the next lesson unlocks. It has
// no code of its own; it never imports Glutter.
package e2e
