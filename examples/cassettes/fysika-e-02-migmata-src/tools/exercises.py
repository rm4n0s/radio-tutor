"""The exercises of the cassette (Greek text for the child, English grading notes)."""

# Each exercise: id, title, label, prompt, reference_solution, grading, hints, show_work, svg (optional prompt.svg)

WORKSHEET_1 = [
    dict(
        id="katastaseis-ylon", title="Στερεό, υγρό ή αέριο;", label="1",
        prompt=("Για κάθε μίγμα γράψε σε ποια φυσική κατάσταση (στερεό, υγρό ή αέριο) βρίσκεται καθένα από τα δύο συστατικά "
                "και σε ποια το μίγμα που προκύπτει.\n"
                "α) άμμος + βότσαλα\n"
                "β) νερό + ζάχαρη (ανακατεμένη στο νερό)\n"
                "γ) νερό + διοξείδιο του άνθρακα (όπως στη σόδα)"),
        reference_solution=("α) άμμος: στερεό, βότσαλα: στερεό, μίγμα: στερεό.\n"
                            "β) νερό: υγρό, ζάχαρη: στερεό, μίγμα: υγρό.\n"
                            "γ) νερό: υγρό, διοξείδιο του άνθρακα: αέριο, μίγμα: υγρό (με φυσαλίδες)."),
        grading=("There are 9 small answers (3 per part: first component, second component, mixture). Correct: all 9 right "
                 "(accept 'στερεά', 'υγρά' as plural, and for (γ) accept 'υγρό' or 'υγρό με φυσαλίδες'). "
                 "Partial: at least 6 of the 9 right. Incorrect: fewer. The child may write the answers in a table or in sentences; "
                 "both are fine. Common mistakes and what to tell the child: (1) writing 'υγρό' for the sugar in β: say that sugar "
                 "is a solid even when we cannot see it in the water; (2) writing 'αέριο' for the soda in γ: say that the bubbles are "
                 "the gas, but the drink as a whole is a liquid; (3) writing 'υγρό' for the sand and pebbles mixture: say that sand "
                 "and pebbles are both solid, so the mixture is solid. Do not penalise spelling or missing accents."),
        hints=["Κοίτα κάθε συστατικό ξεχωριστά. Μπορείς να το πιάσεις και να κρατήσει το σχήμα του;",
               "Η ζάχαρη και η άμμος είναι στερεά. Το νερό είναι υγρό.",
               "Το μίγμα παίρνει συνήθως τη μορφή του κύριου υλικού του. Μια σόδα την πίνουμε, άρα είναι υγρό."],
        show_work=False),
    dict(
        id="kathari-i-migma", title="Καθαρή ουσία ή μίγμα;", label="2",
        prompt=("Χώρισε τα έξι υλικά σε δύο στήλες: «καθαρές ουσίες» και «μίγματα».\n"
                "• ο χαλκός στα σύρματα\n• η σαλάτα\n• το αποσταγμένο νερό\n• το λαδόξιδο\n"
                "• ο υδράργυρος του θερμομέτρου\n• ο χυμός πορτοκαλιού"),
        reference_solution=("Καθαρές ουσίες: ο χαλκός, το αποσταγμένο νερό, ο υδράργυρος.\n"
                            "Μίγματα: η σαλάτα, το λαδόξιδο, ο χυμός πορτοκαλιού."),
        grading=("Correct: all six placed correctly (3 pure substances: copper, distilled water, mercury; 3 mixtures: salad, "
                 "vinaigrette, orange juice). Partial: 4 or 5 placed correctly. Incorrect: 3 or fewer. The child may write two lists, "
                 "a table, or draw lines/arrows; accept any clear form. Common mistakes: (1) putting the distilled water among the "
                 "mixtures: tell the child that distilled water is only water, nothing else is mixed in it; (2) putting the orange "
                 "juice among the pure substances: tell the child that juice has water, sugar and other substances mixed together; "
                 "(3) thinking that metals like copper are mixtures: say that a wire made only of copper has one substance."),
        hints=["Μια καθαρή ουσία έχει μόνο ένα υλικό. Ένα μίγμα έχει δύο ή περισσότερα ανακατεμένα.",
               "Ρώτα τον εαυτό σου: από τι είναι φτιαγμένη η σαλάτα; Είναι ένα υλικό ή πολλά;",
               "Ο χαλκός, ο υδράργυρος και το αποσταγμένο νερό είναι καθαρές ουσίες."],
        show_work=False),
    dict(
        id="swsto-lathos-migmata", title="Σωστό ή λάθος;", label="3",
        prompt=("Γράψε Σ (σωστό) ή Λ (λάθος) για κάθε πρόταση. Αν είναι λάθος, γράψε τη σωστή πρόταση.\n"
                "α) Το αλατόνερο είναι αλμυρό, γιατί το αλάτι είναι αλμυρό.\n"
                "β) Ο καφές έχει σκούρο χρώμα και η ζάχαρη είναι γλυκιά. Ο καφές με ζάχαρη είναι άχρωμος και άγευστος.\n"
                "γ) Ένα μίγμα έχει πάντα μία μόνο ουσία."),
        reference_solution=("α) Σ.\nβ) Λ. Σωστά: ο καφές με ζάχαρη είναι σκούρος και γλυκός (το μίγμα κρατάει τις ιδιότητες των συστατικών του).\n"
                            "γ) Λ. Σωστά: ένα μίγμα έχει δύο ή περισσότερες ουσίες."),
        grading=("Correct: α = Σ, β = Λ with a corrected sentence saying the coffee with sugar is dark and sweet, γ = Λ with a corrected "
                 "sentence saying a mixture has two or more substances. Partial: all three Σ/Λ right but a correction missing or wrong; "
                 "or two of three right. Incorrect: otherwise. Accept any wording that says the same. Do not penalise spelling. "
                 "Common mistake: marking β as Σ: tell the child that a mixture keeps the properties of its ingredients, so the "
                 "coffee stays dark and the sugar still makes it sweet."),
        hints=["Θυμήσου: ένα μίγμα κρατάει τις ιδιότητες των συστατικών του.",
               "Τι χρώμα έχει ο καφές; Τι γεύση έχει η ζάχαρη; Χάνονται όταν τα ανακατέψεις;",
               "Για το γ, μέτρα πόσες ουσίες χρειάζονται για να γίνει ένα μίγμα."],
        show_work=False),
    dict(
        id="etiketa-portokalada", title="Η ετικέτα της πορτοκαλάδας", label="4",
        prompt=("Στην ετικέτα μιας πορτοκαλάδας διαβάζεις: «Συστατικά: νερό, ζάχαρη, χυμός πορτοκαλιού, διοξείδιο του άνθρακα».\n"
                "α) Η πορτοκαλάδα είναι καθαρή ουσία ή μίγμα;\n"
                "β) Πόσα συστατικά γράφει η ετικέτα;\n"
                "γ) Ποιο συστατικό είναι στερεό και ποιο αέριο;\n"
                "δ) Γράψε δύο άλλα μίγματα που υπάρχουν στην κουζίνα."),
        reference_solution=("α) Μίγμα.\nβ) Τέσσερα (4).\nγ) Στερεό: η ζάχαρη. Αέριο: το διοξείδιο του άνθρακα.\n"
                            "δ) Για παράδειγμα: σαλάτα, λαδόξιδο, τσάι, γάλα με κακάο, μαγιονέζα (οποιαδήποτε δύο σωστά μίγματα)."),
        grading=("Correct: α = μίγμα; β = 4 (τέσσερα); γ = ζάχαρη (solid) and διοξείδιο του άνθρακα (gas); δ = two valid mixtures. "
                 "Partial: three of the four parts right. Incorrect: fewer. For δ accept any real mixture from a kitchen (σαλάτα, "
                 "λαδόξιδο, τσάι, χυμός, γάλα με κακάο, αναψυκτικό, σούπα, μαγιονέζα...). Do not accept a single pure substance such "
                 "as νερό, ζάχαρη or αλάτι alone, and do not accept the soft drink from the label again. "
                 "Common mistakes: (1) answering 'καθαρή ουσία' in α: tell the child that the label lists several ingredients, so it is a "
                 "mixture; (2) counting 3 or 5 ingredients in β: tell the child to count the words separated by commas."),
        hints=["Αν η ετικέτα λέει πολλά συστατικά, η πορτοκαλάδα είναι καθαρή ουσία ή μίγμα;",
               "Μέτρα τα συστατικά που χωρίζονται με κόμμα.",
               "Η ζάχαρη είναι σαν κρύσταλλοι ή σκόνη. Το αέριο είναι οι φυσαλίδες."],
        show_work=False),
]

WORKSHEET_2 = [
    dict(
        id="methodoi-diaxorismou", title="Διαλογή ή διήθηση;", label="5",
        prompt=("Για κάθε περίπτωση γράψε ποια μέθοδος διαχωρισμού ταιριάζει: διαλογή ή διήθηση (φιλτράρισμα).\n"
                "α) Βγάζουμε τα πετραδάκια από τις φακές.\n"
                "β) Περνάμε έναν χυμό από το σουρωτήρι, για να μείνουν πάνω τα κομμάτια του φρούτου.\n"
                "γ) Φτιάχνουμε καφέ φίλτρου.\n"
                "δ) Βάζουμε το γυαλί και το χαρτί σε διαφορετικούς κάδους."),
        reference_solution="α) διαλογή\nβ) διήθηση\nγ) διήθηση\nδ) διαλογή",
        grading=("Correct: all four right (α διαλογή, β διήθηση, γ διήθηση, δ διαλογή). Partial: exactly three right. Incorrect: two or fewer. "
                 "Accept 'φιλτράρισμα' for διήθηση. Do not penalise spelling or missing accents. Common mistakes: (1) writing διαλογή for "
                 "the strainer or the coffee filter: tell the child that when a liquid passes through a filter or strainer and the solid "
                 "stays behind, that is διήθηση; (2) writing διήθηση for the stones or the bins: tell the child that when we pick things "
                 "out one by one, that is διαλογή."),
        hints=["Διαλογή: διαλέγουμε και ξεχωρίζουμε με το χέρι. Διήθηση: το υγρό περνάει από ένα φίλτρο.",
               "Στο σουρωτήρι και στον καφέ φίλτρου περνάει ένα υγρό και μένουν στερεά πάνω.",
               "Στα πετραδάκια και στους κάδους ξεχωρίζουμε αντικείμενα ένα ένα."],
        show_work=False),
    dict(
        id="anakinhste-kala", title="Γιατί ανακινούμε;", label="6",
        prompt=("Ένα μπουκάλι σοκολατούχο γάλα έμεινε ακίνητο όλη μέρα.\n"
                "α) Ζωγράφισε δύο μπουκάλια: το ένα όπως είναι πριν ανακινηθεί και το άλλο αμέσως μετά την ανακίνηση. Βάλε μια τελίτσα για κάθε σωματίδιο.\n"
                "β) Πώς λέγεται το στερεό στρώμα που μαζεύεται στον πάτο;\n"
                "γ) Γιατί πρέπει να ανακινήσουμε το μπουκάλι πριν το πιούμε;"),
        reference_solution=("α) Πρώτο μπουκάλι: τα σωματίδια (τελίτσες) μαζεμένα στον πάτο, σε στρώμα. Δεύτερο μπουκάλι: τα σωματίδια διάσπαρτα σε όλο το υγρό.\n"
                            "β) Ίζημα.\n"
                            "γ) Για να ανακατευτεί το ίζημα ξανά με το υγρό και να γίνει το γάλα ομοιόμορφο και γευστικό."),
        grading=("Correct: (α) the first bottle clearly shows dots gathered at the bottom and the second bottle shows dots spread through the "
                 "liquid; (β) 'ίζημα'; (γ) says the sediment has to be mixed with the liquid again (any wording, e.g. 'για να ανακατευτεί "
                 "το ίζημα με το γάλα'). Partial: two of the three parts right. Incorrect: otherwise. The drawing does not have to be beautiful. "
                 "Common mistakes: (1) drawing the dots at the top: tell the child that the solid pieces are heavier and fall to the bottom; "
                 "(2) calling it 'διαλύτης' or 'μίγμα': tell the child the word is ίζημα; (3) answering 'για να είναι κρύο' or 'για να φουσκώσει': "
                 "tell the child that we shake it to mix the sediment with the milk again."),
        hints=["Τα στερεά κομματάκια είναι βαρύτερα από το υγρό. Πού πηγαίνουν όταν το μπουκάλι μένει ακίνητο;",
               "Στο δεύτερο μπουκάλι τα σωματίδια έχουν ανακατευτεί με το υγρό.",
               "Η λέξη που ψάχνεις έχει σχέση με κάτι που «κάθεται» κάτω: ίζ...μα."],
        show_work=False),
    dict(
        id="kadoi-anakyklosis", title="Σε ποιον κάδο;", label="7",
        prompt=("Ζωγράφισε τέσσερις κάδους και γράψε πάνω τους: γυαλί, χαρτί, αλουμίνιο, φαγητό.\n"
                "Σε ποιον κάδο πάει κάθε αντικείμενο; Γράψε το όνομά του κάτω από τον σωστό κάδο.\n"
                "• γυάλινο βάζο\n• εφημερίδα\n• κουτάκι αναψυκτικού από αλουμίνιο\n• φλούδα μπανάνας\n• γυάλινο μπουκάλι\n• χαρτόκουτο"),
        reference_solution=("Κάδος γυαλιού: γυάλινο βάζο, γυάλινο μπουκάλι.\nΚάδος χαρτιού: εφημερίδα, χαρτόκουτο.\n"
                            "Κάδος αλουμινίου: κουτάκι αναψυκτικού.\nΚάδος φαγητού: φλούδα μπανάνας."),
        grading=("Correct: all six objects under the right bin (2 glass, 2 paper, 1 aluminium, 1 food). Partial: four or five right. "
                 "Incorrect: three or fewer. Accept bins drawn as simple boxes, a table, or lists with the bin name as a title; what matters is "
                 "that each object is clearly connected to one bin. Do not penalise spelling or the quality of the drawing. "
                 "Common mistakes: (1) putting the cardboard box with the aluminium or the glass: tell the child that cardboard is made of paper; "
                 "(2) putting the banana peel with the paper: tell the child that food leftovers go to a bin of their own."),
        hints=["Από ποιο υλικό είναι φτιαγμένο κάθε αντικείμενο;",
               "Το βάζο και το μπουκάλι είναι από γυαλί. Η εφημερίδα και το χαρτόκουτο είναι από χαρτί.",
               "Η φλούδα μπανάνας είναι υπόλειμμα φαγητού."],
        show_work=False),
    dict(
        id="swsto-lathos-eterogena", title="Σωστό ή λάθος για τα ετερογενή", label="8",
        prompt=("Γράψε Σ (σωστό) ή Λ (λάθος). Στις λάθος προτάσεις γράψε τη σωστή.\n"
                "α) Σε ένα ετερογενές μίγμα μπορούμε να δούμε τα συστατικά με το μάτι ή με μικροσκόπιο.\n"
                "β) Το ίζημα είναι το στερεό που μαζεύεται στον πάτο του δοχείου.\n"
                "γ) Με τη διήθηση αφαιρούμε τα πετραδάκια από τις φακές με το χέρι.\n"
                "δ) Η φυγοκέντριση και η απόσταξη είναι απλές μέθοδοι, που κάνουμε κάθε μέρα στο σπίτι."),
        reference_solution=("α) Σ\nβ) Σ\nγ) Λ. Σωστά: αυτό λέγεται διαλογή (η διήθηση είναι όταν περνάμε ένα υγρό από φίλτρο).\n"
                            "δ) Λ. Σωστά: είναι πιο σύνθετες μέθοδοι και γίνονται σε ειδικές εγκαταστάσεις."),
        grading=("Correct: α Σ, β Σ, γ Λ with the right correction (it is διαλογή), δ Λ with the right correction (they are complex methods "
                 "used in special facilities). Partial: all four Σ/Λ right but one correction missing or wrong; or three of four Σ/Λ right. "
                 "Incorrect: otherwise. Accept any wording with the same meaning. Do not penalise spelling. Common mistakes: marking γ as Σ: "
                 "tell the child that picking stones out by hand is διαλογή, while διήθηση needs a filter or strainer."),
        hints=["Ετερογενές είναι το μίγμα που βλέπουμε τα συστατικά του.",
               "Θυμήσου ποια μέθοδο χρησιμοποιούμε για τα πετραδάκια στις φακές.",
               "Η φυγοκέντριση, η απόσταξη και η χρωματογραφία χρειάζονται ειδικές εγκαταστάσεις."],
        show_work=False),
]

WORKSHEET_3 = [
    dict(
        id="omogenes-eterogenes", title="Ομογενές ή ετερογενές;", label="9",
        prompt=("Για κάθε μίγμα γράψε αν είναι ομογενές (διάλυμα) ή ετερογενές.\n"
                "α) αλατόνερο\nβ) σαλάτα\nγ) νερό με λάδι\nδ) νερό με ζάχαρη, καλά ανακατεμένο\nε) νερό με χώμα\nστ) καθαρός αέρας"),
        reference_solution=("α) ομογενές\nβ) ετερογενές\nγ) ετερογενές\nδ) ομογενές\nε) ετερογενές\nστ) ομογενές"),
        grading=("Correct: all six right (ομογενή: αλατόνερο, νερό με ζάχαρη, καθαρός αέρας; ετερογενή: σαλάτα, νερό με λάδι, νερό με χώμα). "
                 "Partial: four or five right. Incorrect: three or fewer. Accept 'διάλυμα' for ομογενές. Accept abbreviations such as Ο / Ε "
                 "only if the child clearly defined them; otherwise ask for the words. Do not penalise spelling. Common mistakes: "
                 "(1) calling the sugar water ετερογενές: tell the child that the sugar has dissolved, so we cannot see it any more, and "
                 "then the mixture is ομογενές; (2) calling the air ετερογενές: tell the child that clean air is a solution of gases; "
                 "(3) calling the salad ομογενές: tell the child that we can see every piece."),
        hints=["Ομογενές: δεν μπορούμε να δούμε τα συστατικά, ούτε με μικροσκόπιο. Ετερογενές: τα βλέπουμε.",
               "Όταν ένα στερεό διαλύεται στο νερό, το βλέπουμε ακόμα;",
               "Στη σαλάτα, στο λάδι με το νερό και στο χώμα με το νερό ξεχωρίζουν τα συστατικά."],
        show_work=False),
    dict(
        id="dialytis-kai-dialymena", title="Διαλύτης και διαλυμένη ουσία", label="10",
        prompt=("α) Σε ένα ποτήρι τσάι υπάρχουν πολύ νερό και λίγη ζάχαρη. Ποιος είναι ο διαλύτης και ποια η διαλυμένη ουσία;\n"
                "β) Ένα διάλυμα έχει 90 ml νερό και 10 ml οινόπνευμα. Ποιος είναι ο διαλύτης;\n"
                "γ) Για κάθε διάλυμα γράψε αν είναι υγρό, αέριο ή στερεό: αλατόνερο, καθαρός αέρας, χρυσό κόσμημα (κράμα χρυσού και χαλκού)."),
        reference_solution=("α) Διαλύτης: το νερό. Διαλυμένη ουσία: η ζάχαρη.\nβ) Το νερό (έχει τη μεγαλύτερη ποσότητα).\n"
                            "γ) Αλατόνερο: υγρό. Καθαρός αέρας: αέριο. Χρυσό κόσμημα: στερεό."),
        grading=("Correct: α νερό = διαλύτης, ζάχαρη = διαλυμένη ουσία; β νερό; γ υγρό, αέριο, στερεό (in this order). "
                 "Partial: four or five of the six small answers right. Incorrect: three or fewer. Do not penalise spelling. Common mistakes: "
                 "(1) swapping the two words in α: tell the child that the solvent is the component with the larger quantity; (2) answering "
                 "'οινόπνευμα' in β: tell the child that the solvent is the one with the larger quantity, and 90 ml is more than 10 ml; "
                 "(3) answering 'υγρό' for the golden ring: tell the child that metals mixed together form a solid solution, called κράμα."),
        hints=["Ο διαλύτης είναι το συστατικό που έχει τη μεγαλύτερη ποσότητα.",
               "Η ουσία που διαλύεται, μέσα στον διαλύτη, λέγεται διαλυμένη ουσία.",
               "Στο γ, σκέψου αν μπορείς να αγγίξεις το σώμα, να το πιεις ή να το αναπνεύσεις."],
        show_work=False,
    ),
    dict(
        id="zesto-i-kryo", title="Ζεστό ή κρύο νερό;", label="11",
        prompt=("Η Ελένη έριξε κουταλάκια ζάχαρη σε δύο ποτήρια με την ίδια ποσότητα νερού και ανακάτευε μετά από κάθε κουταλάκι. "
                "Έριχνε μέχρι να μείνει ζάχαρη στον πάτο. Τα αποτελέσματα φαίνονται στην εικόνα.\n"
                "α) Σε ποιο ποτήρι διαλύθηκε περισσότερη ζάχαρη;\n"
                "β) Πόσα κουταλάκια περισσότερα διαλύθηκαν;\n"
                "γ) Εξήγησε γιατί (γράψε μία πρόταση)."),
        reference_solution=("α) Στο ποτήρι με το ζεστό νερό.\nβ) 8 − 3 = 5 κουταλάκια περισσότερα.\n"
                            "γ) Όταν το νερό είναι πιο ζεστό, τα μόρια κινούνται πιο γρήγορα και ανακατεύονται πιο εύκολα με τη ζάχαρη, "
                            "γι' αυτό διαλύεται περισσότερη."),
        grading=("Correct: α the hot-water glass; β 5 (the picture shows 3 spoons for the cold water and 8 spoons for the hot water; accept "
                 "'5 κουταλάκια'); γ a sentence saying that the solvent dissolves more when it is warmer (accept an explanation with faster "
                 "moving molecules, or simply 'όσο πιο ζεστό το νερό, τόσο περισσότερη ζάχαρη διαλύεται'). Partial: two of three right. "
                 "Incorrect: otherwise. Show work is not required for β but accept 8 − 3 = 5. Common mistakes: (1) answering the cold glass: "
                 "tell the child that sugar dissolves more in warm water; (2) answering 11 for β (adding): tell the child to subtract, 8 minus 3; "
                 "(3) explaining with 'γιατί λιώνει': accept it only if it says warm water dissolves more."),
        hints=["Κοίτα πόσα κουταλάκια διαλύθηκαν σε κάθε ποτήρι. Ποιος αριθμός είναι μεγαλύτερος;",
               "Για το β κάνε αφαίρεση: οκτώ μείον τρία.",
               "Στο ζεστό νερό τα μόρια κινούνται πιο γρήγορα. Τι βοηθάει αυτό;"],
        show_work=False, svg="zesto-kryo"),
    dict(
        id="plires-pothari", title="Το ποτήρι που γέμισε ίζημα", label="12",
        prompt=("Η Μαρία ρίχνει κουταλιές αλάτι σε ένα μισογεμάτο ποτήρι νερό και ανακατεύει. Μετά από λίγες κουταλιές το αλάτι δε διαλύεται πια και μένει στον πάτο.\n"
                "α) Πώς λέγεται αυτό που μένει στον πάτο;\n"
                "β) Ποιο είναι ο διαλύτης και ποια η διαλυμένη ουσία;\n"
                "γ) Η Μαρία κάνει το ίδιο με ένα γεμάτο ποτήρι (ίδια θερμοκρασία). Θα διαλυθούν περισσότερες ή λιγότερες κουταλιές αλάτι; Γράψε γιατί."),
        reference_solution=("α) Ίζημα.\nβ) Διαλύτης: το νερό. Διαλυμένη ουσία: το αλάτι.\n"
                            "γ) Περισσότερες, γιατί υπάρχει περισσότερος διαλύτης (περισσότερο νερό) και έτσι χωράει περισσότερο αλάτι."),
        grading=("Correct: α ίζημα; β νερό = διαλύτης, αλάτι = διαλυμένη ουσία; γ 'περισσότερες' with a reason mentioning that there is more water "
                 "(more solvent). Partial: two of three right, or all three answers right but the reason in γ missing. Incorrect: otherwise. "
                 "Do not penalise spelling. Common mistakes: (1) writing 'λιγότερες' in γ: tell the child that the more solvent we have, the "
                 "more of the substance can be dissolved; (2) naming the sediment 'διάλυμα' or 'διαλύτης': tell the child the settled amount is called ίζημα; "
                 "(3) swapping solvent and dissolved substance in β: tell the child that the water has the larger quantity."),
        hints=["Η ποσότητα που δε διαλύεται και κατακάθεται έχει ένα ειδικό όνομα. Ξεκινάει από ί.",
               "Ο διαλύτης είναι το συστατικό που έχει τη μεγαλύτερη ποσότητα.",
               "Αν το ποτήρι έχει περισσότερο νερό, χωράει περισσότερο αλάτι;"],
        show_work=False, svg="pothari"),
]
