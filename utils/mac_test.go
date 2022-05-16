package utils

import (
	"crypto/rand"
	"testing"
)

func BenchmarkHmacSign(b *testing.B) {
	signer := NewSWCertifier()

	var message [256000]byte
	rand.Read(message[:])

	b.SetParallelism(8)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = signer.CreateSignedCertificate(message[:], 0)
	}
}

func BenchmarkHmacVerify(b *testing.B) {
	signer := NewSWCertifier()

	var message [25600]byte
	rand.Read(message[:])

	cert := signer.CreateSignedCertificate(message[:], 0)

	b.SetParallelism(8)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = signer.VerifyCertificate(message[:], 0, cert)
	}
}

func BenchmarkPBFTSimulation(b *testing.B) {
	signer := NewSWCertifier()
	count := 193

	var proposal [25600]byte
	rand.Read(proposal[:])

	var ack [48]byte
	rand.Read(ack[:])

	b.SetParallelism(8)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var cert []byte
		for i := 0; i < count; i++ {
			cert = signer.CreateSignedCertificate(proposal[:], 0)
		}
		_ = signer.VerifyCertificate(proposal[:], 0, cert)

		for i := 0; i < count; i++ {
			cert = signer.CreateSignedCertificate(ack[:], 0)
		}
		_ = signer.VerifyCertificate(ack[:], 0, cert)

		for i := 0; i < count; i++ {
			cert = signer.CreateSignedCertificate(ack[:], 0)
		}
		_ = signer.VerifyCertificate(ack[:], 0, cert)
	}
}

func BenchmarkPBFTSimulationParallel(b *testing.B) {
	count := 193

	var proposal [25600]byte
	rand.Read(proposal[:])

	var ack [48]byte
	rand.Read(ack[:])

	b.ResetTimer()
	b.RunParallel(func(p *testing.PB) {
		signer := NewSWCertifier()
		for p.Next() {
			var cert []byte
			for i := 0; i < count; i++ {
				cert = signer.CreateSignedCertificate(proposal[:], 0)
			}
			_ = signer.VerifyCertificate(proposal[:], 0, cert)

			for i := 0; i < count; i++ {
				cert = signer.CreateSignedCertificate(ack[:], 0)
			}
			_ = signer.VerifyCertificate(ack[:], 0, cert)

			for i := 0; i < count; i++ {
				cert = signer.CreateSignedCertificate(ack[:], 0)
			}
			_ = signer.VerifyCertificate(ack[:], 0, cert)
		}
	})
}
