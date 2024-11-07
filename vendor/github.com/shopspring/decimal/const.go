package decimal

import (
	"strings"
)

const (
	strLn10 = "2.302585092994045684017991454684364207601101488628772976033327900967572609677352480235997205089598298341967784042286248633409525465082806756666287369098781689482907208325554680843799894826233198528393505308965377732628846163366222287698219886746543667474404243274365155048934314939391479619404400222105101714174800368808401264708068556774321622835522011480466371565912137345074785694768346361679210180644507064800027750268491674655058685693567342067058113642922455440575892572420824131469568901675894025677631135691929203337658714166023010570308963457207544037084746994016826928280848118428931484852494864487192780967627127577539702766860595249671667418348570442250719796500471495105049221477656763693866297697952211071826454973477266242570942932258279850258550978526538320760672631716430950599508780752371033310119785754733154142180842754386359177811705430982748238504564801909561029929182431823752535770975053956518769751037497088869218020518933950723853920514463419726528728696511086257149219884997874887377134568620916705849807828059751193854445009978131146915934666241071846692310107598438319191292230792503747298650929009880391941702654416816335727555703151596113564846546190897042819763365836983716328982174407366009162177850541779276367731145041782137660111010731042397832521894898817597921798666394319523936855916447118246753245630912528778330963604262982153040874560927760726641354787576616262926568298704957954913954918049209069438580790032763017941503117866862092408537949861264933479354871737451675809537088281067452440105892444976479686075120275724181874989395971643105518848195288330746699317814634930000321200327765654130472621883970596794457943468343218395304414844803701305753674262153675579814770458031413637793236291560128185336498466942261465206459942072917119370602444929358037007718981097362533224548366988505528285966192805098447175198503666680874970496982273220244823343097169111136813588418696549323714996941979687803008850408979618598756579894836445212043698216415292987811742973332588607915912510967187510929248475023930572665446276200923068791518135803477701295593646298412366497023355174586195564772461857717369368404676577047874319780573853271810933883496338813069945569399346101090745616033312247949360455361849123333063704751724871276379140924398331810164737823379692265637682071706935846394531616949411701841938119405416449466111274712819705817783293841742231409930022911502362192186723337268385688273533371925103412930705632544426611429765388301822384091026198582888433587455960453004548370789052578473166283701953392231047527564998119228742789713715713228319641003422124210082180679525276689858180956119208391760721080919923461516952599099473782780648128058792731993893453415320185969711021407542282796298237068941764740642225757212455392526179373652434440560595336591539160312524480149313234572453879524389036839236450507881731359711238145323701508413491122324390927681724749607955799151363982881058285740538000653371655553014196332241918087621018204919492651483892"
)

var (
	ln10 = newConstApproximation(strLn10)
)

type constApproximation struct {
	exact          Decimal
	approximations []Decimal
}

func newConstApproximation(value string) constApproximation {
	parts := strings.Split(value, ".")
	coeff, fractional := parts[0], parts[1]

	coeffLen := len(coeff)
	maxPrecision := len(fractional)

	var approximations []Decimal
	for p := 1; p < maxPrecision; p *= 2 {
		r := RequireFromString(value[:coeffLen+p])
		approximations = append(approximations, r)
	}

	return constApproximation{
		RequireFromString(value),
		approximations,
	}
}

// Returns the smallest approximation available that's at least as precise
// as the passed precision (places after decimal point), i.e. Floor[ log2(precision) ] + 1
func (c constApproximation) withPrecision(precision int32) Decimal {
	i := 0

	if precision >= 1 {
		i++
	}

	for precision >= 16 {
		precision /= 16
		i += 4
	}

	for precision >= 2 {
		precision /= 2
		i++
	}

	if i >= len(c.approximations) {
		return c.exact
	}

	return c.approximations[i]
}