package ch.epfl.dedis.lib.omniledger.darc;

import ch.epfl.dedis.lib.exception.CothorityAlreadyExistsException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import ch.epfl.dedis.proto.DarcProto;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.stream.Collectors;

public class Rules {
    private List<Rule> r;

    public Rules() {
        this.r = new ArrayList<>();
    }

    public Rules(Rules other) {
        List<Rule> newList = new ArrayList<>(other.r.size());
        newList.addAll(other.r);
        this.r = newList;
    }

    public Rules(DarcProto.Rules rules) {
        this.r = new ArrayList<>();
        for (DarcProto.Rule protoRule : rules.getRList()) {
            Rule r = new Rule(protoRule.getAction(), protoRule.getExpr().toByteArray());
            this.r.add(r);
        }
    }

    public void addRule(String a, byte[] expr) throws CothorityAlreadyExistsException {
        if (exists(a) != -1) {
            throw new CothorityAlreadyExistsException("rule already exists");
        }
        r.add(new Rule(a, expr));
    }

    public void updateRule(String a, byte[] expr) throws CothorityNotFoundException {
        int i = exists(a);
        if (i == -1) {
            throw new CothorityNotFoundException("cannot update a non-existing rule");
        }
        this.r.set(i, new Rule(a, expr));
    }

    public Rule get(String a) {
        for (Rule rule : this.r) {
            if (rule.getAction().equals(a)) {
                return rule;
            }
        }
        return null;
    }

    public List<Rule> getAllRules() {
        return this.r;
    }

    public List<String> getAllActions() {
        return this.r.stream().map(Rule::getAction).collect(Collectors.toList());
    }

    public Rule remove(String a) {
        int i = exists(a);
        if (i == -1) {
            return null;
        }
        return this.r.remove(i);
    }

    public boolean contains(String a) {
        return exists(a) != -1;
    }

    public DarcProto.Rules toProto() {
        DarcProto.Rules.Builder b = DarcProto.Rules.newBuilder();
        for (Rule rule : this.r) {
            b.addR(rule.toProto());
        }
        return b.build();
    }

    private int exists(String a) {
        for (int i = 0; i < r.size(); i++) {
            if (r.get(i).getAction().equals(a)) {
                return i;
            }
        }
        return -1;
    }
}
