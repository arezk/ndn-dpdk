package cs_test

import (
	"testing"
	"time"

	"github.com/usnistgov/ndn-dpdk/container/cs"
	"github.com/usnistgov/ndn-dpdk/container/pcct"
	"github.com/usnistgov/ndn-dpdk/dpdk/eal"
	"github.com/usnistgov/ndn-dpdk/ndni"
	"github.com/usnistgov/ndn-dpdk/ndni/ndntestenv"
)

func TestInsertErase(t *testing.T) {
	assert, require := makeAR(t)
	var cfg pcct.Config
	fixture := NewFixture(cfg)
	defer fixture.Close()

	ok := fixture.Insert(makeInterest("/A/B"),
		makeData("/A/B"))
	assert.True(ok)
	assert.Equal(1, fixture.Cs.CountEntries(cs.CSL_MD))
	assert.Zero(fixture.Pit.Len())
	assert.Equal(1, fixture.CountMpInUse())

	csEntry := fixture.Find(makeInterest("/A/B"))
	require.NotNil(csEntry)
	assert.False(csEntry.IsFresh(eal.TscNow()))

	ok = fixture.Insert(makeInterest("/A/B", ndni.MustBeFreshFlag),
		makeData("/A/B", 100*time.Millisecond))
	assert.True(ok)
	assert.Equal(1, fixture.Cs.CountEntries(cs.CSL_MD))

	csEntry = fixture.Find(makeInterest("/A/B"))
	require.NotNil(csEntry)
	csData := csEntry.GetData()
	ndntestenv.NameEqual(assert, "/A/B", csData)
	assert.Equal(100*time.Millisecond, csData.GetFreshnessPeriod())

	ok = fixture.Insert(
		makeInterest("/A/B", ndni.FHDelegation{1, "/F"}, ndni.ActiveFHDelegation(0)),
		makeData("/A/B", 200*time.Millisecond))
	assert.True(ok)
	assert.Equal(2, fixture.Cs.CountEntries(cs.CSL_MD))

	csEntry3 := fixture.Find(makeInterest("/A/B",
		ndni.FHDelegation{1, "/G"}, ndni.FHDelegation{2, "/F"}, ndni.ActiveFHDelegation(1)))
	require.NotNil(csEntry3)
	csData3 := csEntry3.GetData()
	ndntestenv.NameEqual(assert, "/A/B", csData3)
	assert.Equal(200*time.Millisecond, csData3.GetFreshnessPeriod())

	time.Sleep(10 * time.Millisecond)
	assert.NotNil(fixture.Find(makeInterest("/A/B", ndni.MustBeFreshFlag)))
	time.Sleep(120 * time.Millisecond)
	assert.Nil(fixture.Find(makeInterest("/A/B", ndni.MustBeFreshFlag)))
	assert.NotNil(fixture.Find(makeInterest("/A/B")))

	fixture.Cs.Erase(*csEntry)
	fixture.Cs.Erase(*csEntry3)
	assert.Zero(fixture.Cs.CountEntries(cs.CSL_MD))
	assert.Zero(fixture.CountMpInUse())
}

func TestPrefixMatch(t *testing.T) {
	assert, require := makeAR(t)
	var cfg pcct.Config
	fixture := NewFixture(cfg)
	defer fixture.Close()

	// /A/B/C/D <- [/A/B]
	ok := fixture.Insert(makeInterest("/A/B", ndni.CanBePrefixFlag),
		makeData("/A/B/C/D"))
	assert.True(ok)
	assert.Equal(1, fixture.Cs.CountEntries(cs.CSL_MD))
	assert.Equal(1, fixture.Cs.CountEntries(cs.CSL_MI))

	direct := fixture.Find(makeInterest("/A/B/C/D"))
	require.NotNil(direct)
	assert.True(direct.IsDirect())
	assert.Len(direct.ListIndirects(), 1)

	indirect2 := fixture.Find(makeInterest("/A/B", ndni.CanBePrefixFlag))
	require.NotNil(indirect2)
	assert.False(indirect2.IsDirect())

	indirect3 := fixture.Find(makeInterest("/A/B/C", ndni.CanBePrefixFlag))
	assert.Nil(indirect3)

	// /A/B/C/D <- [/A/B, /A/B/C]
	ok = fixture.Insert(makeInterest("/A/B/C", ndni.CanBePrefixFlag),
		makeData("/A/B/C/D"))
	assert.True(ok)
	assert.Equal(1, fixture.Cs.CountEntries(cs.CSL_MD))
	assert.Equal(2, fixture.Cs.CountEntries(cs.CSL_MI))

	indirect2 = fixture.Find(makeInterest("/A/B", ndni.CanBePrefixFlag))
	require.NotNil(indirect2)
	assert.False(indirect2.IsDirect())

	indirect3 = fixture.Find(makeInterest("/A/B/C", ndni.CanBePrefixFlag))
	require.NotNil(indirect3)
	assert.False(indirect3.IsDirect())
	assert.Len(direct.ListIndirects(), 2)

	assert.Nil(fixture.Find(makeInterest("/A/B"))) // no match due to CanBePrefix=0
	assert.Equal(1, fixture.Cs.CountEntries(cs.CSL_MD))
	assert.Equal(2, fixture.Cs.CountEntries(cs.CSL_MI))
	assert.Len(direct.ListIndirects(), 2)

	fixture.Cs.Erase(*direct)
	assert.Equal(0, fixture.Cs.CountEntries(cs.CSL_MD))
	assert.Equal(0, fixture.Cs.CountEntries(cs.CSL_MI))

	// /A/B/C/D <- [/A/B] with fh=/F
	ok = fixture.Insert(
		makeInterest("/A/B", ndni.CanBePrefixFlag,
			ndni.FHDelegation{1, "/F"}, ndni.ActiveFHDelegation(0)),
		makeData("/A/B/C/D"))
	assert.True(ok)
	assert.Equal(1, fixture.Cs.CountEntries(cs.CSL_MD))
	assert.Equal(1, fixture.Cs.CountEntries(cs.CSL_MI))

	// /A/B/C/D <- [/A/B, /A/B/C] with fh=/F
	ok = fixture.Insert(
		makeInterest("/A/B/C", ndni.CanBePrefixFlag,
			ndni.FHDelegation{1, "/F"}, ndni.ActiveFHDelegation(0)),
		makeData("/A/B/C/D"))
	assert.True(ok)
	assert.Equal(1, fixture.Cs.CountEntries(cs.CSL_MD))
	assert.Equal(2, fixture.Cs.CountEntries(cs.CSL_MI))

	assert.Nil(fixture.Find(
		makeInterest("/A/B", ndni.CanBePrefixFlag))) // no match due to missing fh=/F

	indirect2 = fixture.Find(
		makeInterest("/A/B", ndni.CanBePrefixFlag,
			ndni.FHDelegation{1, "/F"}, ndni.ActiveFHDelegation(0)))
	require.NotNil(indirect2)
	assert.False(indirect2.IsDirect())
}

func TestImplicitDigestMatch(t *testing.T) {
	assert, _ := makeAR(t)
	var cfg pcct.Config
	fixture := NewFixture(cfg)
	defer fixture.Close()

	// /A/B/C/D {0x01} <- [/A/B]
	data01 := makeData("/A/B/C/D", []byte{0x01})
	fullName01 := data01.GetFullName().String()
	ok := fixture.Insert(makeInterest("/A/B", ndni.CanBePrefixFlag), data01)
	assert.True(ok)
	assert.Equal(1, fixture.Cs.CountEntries(cs.CSL_MD))
	assert.Equal(1, fixture.Cs.CountEntries(cs.CSL_MI))

	// /A/B/C/D {0x01} <- [/A/B, /A/B/C/D/implicit-digest-01]
	data01 = makeData("/A/B/C/D", []byte{0x01})
	assert.Equal(fullName01, data01.GetFullName().String())
	ok = fixture.Insert(makeInterest(fullName01), data01)
	assert.True(ok)
	assert.Equal(1, fixture.Cs.CountEntries(cs.CSL_MD))
	assert.Equal(2, fixture.Cs.CountEntries(cs.CSL_MI))

	assert.NotNil(fixture.Find(makeInterest("/A/B/C/D")))
	assert.NotNil(fixture.Find(makeInterest("/A/B", ndni.CanBePrefixFlag)))
	assert.NotNil(fixture.Find(makeInterest(fullName01)))
	assert.NotNil(fixture.Find(makeInterest(fullName01, ndni.CanBePrefixFlag)))

	// /A/B/C/D {0x02} <- [/A/B, /A/B/C/D/implicit-digest-02]
	data02 := makeData("/A/B/C/D", []byte{0x02})
	fullName02 := data02.GetFullName().String()
	assert.NotEqual(fullName01, fullName02)
	ok = fixture.Insert(makeInterest(fullName02), data02)
	assert.True(ok)
	assert.Equal(1, fixture.Cs.CountEntries(cs.CSL_MD))
	assert.Equal(2, fixture.Cs.CountEntries(cs.CSL_MI))

	assert.NotNil(fixture.Find(makeInterest("/A/B/C/D")))
	assert.NotNil(fixture.Find(makeInterest("/A/B", ndni.CanBePrefixFlag)))
	assert.NotNil(fixture.Find(makeInterest(fullName02)))
	assert.NotNil(fixture.Find(makeInterest(fullName02, ndni.CanBePrefixFlag)))
	assert.Nil(fixture.Find(makeInterest(fullName01)))
	assert.Nil(fixture.Find(makeInterest(fullName01, ndni.CanBePrefixFlag)))
}
